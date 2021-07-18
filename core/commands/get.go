package commands

import (
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	gopath "path"
	"path/filepath"
	"strings"

	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	"github.com/ipfs/go-ipfs/core/commands/e"

	"github.com/cheggaaa/pb"
	cmds "github.com/ipfs/go-ipfs-cmds"
	files "github.com/ipfs/go-ipfs-files"
	"github.com/ipfs/interface-go-ipfs-core/path"
	"github.com/ipfs/tar-utils"
)

var ErrInvalidCompressionLevel = errors.New("compression level must be between 1 and 9")

const (
	outputOptionName           = "output"
	archiveOptionName          = "archive"
	compressOptionName         = "compress"
	compressionLevelOptionName = "compression-level"
)

var GetCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Download IPFS objects.",
		ShortDescription: `
Stores to disk the data contained an IPFS or IPNS object(s) at the given path.

By default, the output will be stored at './<ipfs-path>', but an alternate
path can be specified with '--output=<path>' or '-o=<path>'.

To output a TAR archive instead of unpacked files, use '--archive' or '-a'.

To compress the output with GZIP compression, use '--compress' or '-C'. You
may also specify the level of compression by specifying '-l=<1-9>'.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, false, "The path to the IPFS object(s) to be outputted.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(outputOptionName, "o", "The path where the output should be stored."),
		cmds.BoolOption(archiveOptionName, "a", "Output a TAR archive."),
		cmds.BoolOption(compressOptionName, "C", "Compress the output with GZIP compression."),
		cmds.IntOption(compressionLevelOptionName, "l", "The level of compression (1-9)."),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		_, err := getCompressOptions(req)
		return err
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		cmplvl, err := getCompressOptions(req)
		if err != nil {
			return err
		}

		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		p := path.New(req.Arguments[0])

		file, err := api.Unixfs().Get(req.Context, p)
		if err != nil {
			return err
		}

		size, err := file.Size()
		if err != nil {
			return err
		}

		res.SetLength(uint64(size))

		archive, _ := req.Options[archiveOptionName].(bool)
		reader, err := fileArchive(file, p.String(), getOutPath(req), archive, cmplvl)
		if err != nil {
			return err
		}

		return res.Emit(reader)
	},
	PostRun: cmds.PostRunMap{
		cmds.CLI: func(res cmds.Response, re cmds.ResponseEmitter) error {
			req := res.Request()

			v, err := res.Next()
			if err != nil {
				return err
			}

			outReader, ok := v.(io.Reader)
			if !ok {
				return e.New(e.TypeErr(outReader, v))
			}

			outPath := getOutPath(req)

			cmplvl, err := getCompressOptions(req)
			if err != nil {
				return err
			}

			archive, _ := req.Options[archiveOptionName].(bool)

			gw := getWriter{
				Out:         os.Stdout,
				Err:         os.Stderr,
				Archive:     archive,
				Compression: cmplvl,
				Size:        int64(res.Length()),
			}

			return gw.Write(outReader, outPath)
		},
	},
}

type clearlineReader struct {
	io.Reader
	out io.Writer
}

func (r *clearlineReader) Read(p []byte) (n int, err error) {
	n, err = r.Reader.Read(p)
	if err == io.EOF {
		// callback
		fmt.Fprintf(r.out, "\033[2K\r") // clear progress bar line on EOF
	}
	return
}

func progressBarForReader(out io.Writer, r io.Reader, l int64) (*pb.ProgressBar, io.Reader) {
	bar := makeProgressBar(out, l)
	barR := bar.NewProxyReader(r)
	return bar, &clearlineReader{barR, out}
}

func makeProgressBar(out io.Writer, l int64) *pb.ProgressBar {
	// setup bar reader
	// TODO: get total length of files
	bar := pb.New64(l).SetUnits(pb.U_BYTES)
	bar.Output = out

	// the progress bar lib doesn't give us a way to get the width of the output,
	// so as a hack we just use a callback to measure the output, then get rid of it
	bar.Callback = func(line string) {
		terminalWidth := len(line)
		bar.Callback = nil
		log.Infof("terminal width: %v\n", terminalWidth)
	}
	return bar
}

func getOutPath(req *cmds.Request) string {
	outPath, _ := req.Options[outputOptionName].(string)
	if outPath == "" {
		trimmed := strings.TrimRight(req.Arguments[0], "/")
		_, outPath = filepath.Split(trimmed)
		outPath = filepath.Clean(outPath)
	}
	return outPath
}

type getWriter struct {
	Out io.Writer // for output to user
	Err io.Writer // for progress bar output

	Archive     bool
	Compression int
	Size        int64
}

func (gw *getWriter) Write(r io.Reader, fpath string) error {
	if gw.Archive || gw.Compression != gzip.NoCompression {
		return gw.writeArchive(r, fpath)
	}
	return gw.writeExtracted(r, fpath)
}

func (gw *getWriter) writeArchive(r io.Reader, fpath string) error {
	// adjust file name if tar
	if gw.Archive {
		if !strings.HasSuffix(fpath, ".tar") && !strings.HasSuffix(fpath, ".tar.gz") {
			fpath += ".tar"
		}
	}

	// adjust file name if gz
	if gw.Compression != gzip.NoCompression {
		if !strings.HasSuffix(fpath, ".gz") {
			fpath += ".gz"
		}
	}

	// create file
	file, err := os.Create(fpath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(gw.Out, "Saving archive to %s\n", fpath)
	bar, barR := progressBarForReader(gw.Err, r, gw.Size)
	bar.Start()
	defer bar.Finish()

	_, err = io.Copy(file, barR)
	return err
}

func (gw *getWriter) writeExtracted(r io.Reader, fpath string) error {
	fmt.Fprintf(gw.Out, "Saving file(s) to %s\n", fpath)
	bar := makeProgressBar(gw.Err, gw.Size)
	bar.Start()
	defer bar.Finish()
	defer bar.Set64(gw.Size)

	extractor := &tar.Extractor{Path: fpath, Progress: bar.Add64}
	return extractor.Extract(r)
}

func getCompressOptions(req *cmds.Request) (int, error) {
	cmprs, _ := req.Options[compressOptionName].(bool)
	cmplvl, cmplvlFound := req.Options[compressionLevelOptionName].(int)
	switch {
	case !cmprs:
		return gzip.NoCompression, nil
	case cmprs && !cmplvlFound:
		return gzip.DefaultCompression, nil
	case cmprs && (cmplvl < 1 || cmplvl > 9):
		return gzip.NoCompression, ErrInvalidCompressionLevel
	}
	return cmplvl, nil
}

// DefaultBufSize is the buffer size for gets. for now, 1MB, which is ~4 blocks.
// TODO: does this need to be configurable?
var DefaultBufSize = 1048576

type identityWriteCloser struct {
	w io.Writer
}

func (i *identityWriteCloser) Write(p []byte) (int, error) {
	return i.w.Write(p)
}

func (i *identityWriteCloser) Close() error {
	return nil
}

func updateFileMeta(fn files.Node, filename string) error {
	if t := fn.ModTime(); !t.IsZero() {
		if err := os.Chtimes(filename, t, t); err != nil {
			return err
		}
	}
	if mode := fn.Mode(); mode != 0 {
		if err := os.Chmod(filename, mode); err != nil {
			return err
		}
	}
	return nil
}

func fileArchive(f files.Node, name string, outpath string, archive bool, compression int) (io.Reader, error) {
	var err error = nil
	cleaned := gopath.Clean(name)
	_, filename := gopath.Split(cleaned)

	// need to connect a writer to a reader
	piper, pipew := io.Pipe()
	checkErrAndClosePipe := func(err error) bool {
		if err != nil {
			_ = pipew.CloseWithError(err)
			return true
		}
		return false
	}

	// use a buffered writer to parallelize task
	bufw := bufio.NewWriterSize(pipew, DefaultBufSize)

	// compression determines whether to use gzip compression.
	maybeGzw, err := newMaybeGzWriter(bufw, compression)
	if checkErrAndClosePipe(err) {
		return nil, err
	}

	closeGzwAndPipe := func() error {
		if err := maybeGzw.Close(); checkErrAndClosePipe(err) {
			return nil
		}
		if err := bufw.Flush(); checkErrAndClosePipe(err) {
			return nil
		}
		pipew.Close() // everything seems to be ok.
		if !archive {
			if err := updateFileMeta(f, outpath); err != nil {
				return err
			}
		}
		return nil
	}

	if !archive && compression != gzip.NoCompression {
		// the case when the node is a file
		r := files.ToFile(f)
		if r == nil {
			return nil, errors.New("file is not regular")
		}

		go func() {
			if _, err := io.Copy(maybeGzw, r); checkErrAndClosePipe(err) {
				return
			}
			err = closeGzwAndPipe() // everything seems to be ok
		}()
	} else {
		// the case for 1. archive, and 2. not archived and not compressed, in which tar is used anyway as a transport format

		// construct the tar writer
		w, err := files.NewTarWriter(maybeGzw)
		if checkErrAndClosePipe(err) {
			return nil, err
		}

		go func() {
			// write all the nodes recursively
			if err := w.WriteFile(f, filename); checkErrAndClosePipe(err) {
				return
			}
			w.Close()               // close tar writer
			err = closeGzwAndPipe() // everything seems to be ok
		}()
	}

	return piper, err
}

func newMaybeGzWriter(w io.Writer, compression int) (io.WriteCloser, error) {
	if compression != gzip.NoCompression {
		return gzip.NewWriterLevel(w, compression)
	}
	return &identityWriteCloser{w}, nil
}
