package http

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/rs/cors"

	cmds "github.com/ipfs/go-ipfs/commands"
	u "github.com/ipfs/go-ipfs/util"
)

var log = u.Logger("commands/http")

// the internal handler for the API
type internalHandler struct {
	ctx  cmds.Context
	root *cmds.Command
}

// The Handler struct is funny because we want to wrap our internal handler
// with CORS while keeping our fields.
type Handler struct {
	internalHandler
	corsHandler http.Handler
}

var ErrNotFound = errors.New("404 page not found")

const (
	streamHeader           = "X-Stream-Output"
	channelHeader          = "X-Chunked-Output"
	contentTypeHeader      = "Content-Type"
	contentLengthHeader    = "Content-Length"
	transferEncodingHeader = "Transfer-Encoding"
	applicationJson        = "application/json"
)

var mimeTypes = map[string]string{
	cmds.JSON: "application/json",
	cmds.XML:  "application/xml",
	cmds.Text: "text/plain",
}

func NewHandler(ctx cmds.Context, root *cmds.Command, allowedOrigin string) *Handler {
	// allow whitelisted origins (so we can make API requests from the browser)
	if len(allowedOrigin) > 0 {
		log.Info("Allowing API requests from origin: " + allowedOrigin)
	}

	// Create a handler for the API.
	internal := internalHandler{ctx, root}

	// Create a CORS object for wrapping the internal handler.
	c := cors.New(cors.Options{
		AllowedMethods: []string{"GET", "POST", "PUT"},

		// use AllowOriginFunc instead of AllowedOrigins because we want to be
		// restrictive by default.
		AllowOriginFunc: func(origin string) bool {
			return (allowedOrigin == "*") || (origin == allowedOrigin)
		},
	})

	// Wrap the internal handler with CORS handling-middleware.
	return &Handler{internal, c.Handler(internal)}
}

func (i internalHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Debug("Incoming API request: ", r.URL)

	// error on external referers (to prevent CSRF attacks)
	referer := r.Referer()
	scheme := r.URL.Scheme
	if len(scheme) == 0 {
		scheme = "http"
	}
	host := fmt.Sprintf("%s://%s/", scheme, r.Host)
	// empty string means the user isn't following a link (they are directly typing in the url)
	if referer != "" && !strings.HasPrefix(referer, host) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("403 - Forbidden"))
		return
	}

	req, err := Parse(r, i.root)
	if err != nil {
		if err == ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
		} else {
			w.WriteHeader(http.StatusBadRequest)
		}
		w.Write([]byte(err.Error()))
		return
	}

	// get the node's context to pass into the commands.
	node, err := i.ctx.GetNode()
	if err != nil {
		err = fmt.Errorf("cmds/http: couldn't GetNode(): %s", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	//ps: take note of the name clash - commands.Context != context.Context
	req.SetInvocContext(i.ctx)
	err = req.SetRootContext(node.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if cn, ok := w.(http.CloseNotifier); ok {
		go func() {
			select {
			case <-cn.CloseNotify():
				cancel()
			case <-ctx.Done():
			}
		}()
	}

	// call the command
	res := i.root.Call(req)

	// set the Content-Type based on res output
	if _, ok := res.Output().(io.Reader); ok {
		// we don't set the Content-Type for streams, so that browsers can MIME-sniff the type themselves
		// we set this header so clients have a way to know this is an output stream
		// (not marshalled command output)
		// TODO: set a specific Content-Type if the command response needs it to be a certain type
		w.Header().Set(streamHeader, "1")

	} else {
		enc, found, err := req.Option(cmds.EncShort).String()
		if err != nil || !found {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		mime := mimeTypes[enc]
		w.Header().Set(contentTypeHeader, mime)
	}

	// set the Content-Length from the response length
	if res.Length() > 0 {
		w.Header().Set(contentLengthHeader, strconv.FormatUint(res.Length(), 10))
	}

	// if response contains an error, write an HTTP error status code
	if e := res.Error(); e != nil {
		if e.Code == cmds.ErrClient {
			w.WriteHeader(http.StatusBadRequest)
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}

	out, err := res.Reader()
	if err != nil {
		w.Header().Set(contentTypeHeader, "text/plain")
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(err.Error()))
		return
	}

	// if output is a channel and user requested streaming channels,
	// use chunk copier for the output
	_, isChan := res.Output().(chan interface{})
	if !isChan {
		_, isChan = res.Output().(<-chan interface{})
	}

	streamChans, _, _ := req.Option("stream-channels").Bool()
	if isChan && streamChans {
		// w.WriteString(transferEncodingHeader + ": chunked\r\n")
		// w.Header().Set(channelHeader, "1")
		// w.WriteHeader(200)
		err = copyChunks(applicationJson, w, out)
		if err != nil {
			log.Debug("copy chunks error: ", err)
		}
		return
	}

	err = flushCopy(w, out)
	if err != nil {
		log.Debug("Flush copy returned an error: ", err)
	}
}

func (i Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Call the CORS handler which wraps the internal handler.
	i.corsHandler.ServeHTTP(w, r)
}

// flushCopy Copies from an io.Reader to a http.ResponseWriter.
// Flushes chunks over HTTP stream as they are read (if supported by transport).
func flushCopy(w http.ResponseWriter, out io.Reader) error {
	if _, ok := w.(http.Flusher); !ok {
		return copyChunks("", w, out)
	}

	_, err := io.Copy(&flushResponse{w}, out)
	return err
}

// Copies from an io.Reader to a http.ResponseWriter.
// Flushes chunks over HTTP stream as they are read (if supported by transport).
func copyChunks(contentType string, w http.ResponseWriter, out io.Reader) error {
	if contentType != "" {
		w.Header().Set(contentTypeHeader, contentType)
	}

	w.Header().Set(transferEncodingHeader, "chunked")
	w.Header().Set(channelHeader, "1")

	_, err := io.Copy(w, out)
	if err != nil {
		return err
	}

	return nil
}

type flushResponse struct {
	W http.ResponseWriter
}

func (fr *flushResponse) Write(buf []byte) (int, error) {
	n, err := fr.W.Write(buf)
	if err != nil {
		return n, err
	}

	if flusher, ok := fr.W.(http.Flusher); ok {
		flusher.Flush()
	}
	return n, err
}
