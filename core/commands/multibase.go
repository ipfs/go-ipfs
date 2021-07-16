package commands

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"strings"

	cmds "github.com/ipfs/go-ipfs-cmds"
	"github.com/ipfs/go-ipfs/core/commands/cmdenv"
	mbase "github.com/multiformats/go-multibase"
)

var MbaseCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Encode and decode files or stdin with multibase format",
	},
	Subcommands: map[string]*cmds.Command{
		"encode": mbaseEncodeCmd,
		"decode": mbaseDecodeCmd,
		"list":   basesCmd,
	},
	Extra: CreateCmdExtras(SetDoesNotUseRepo(true)),
}

const (
	mbaseOptionName = "b"
)

var mbaseEncodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Encode data into multibase string",
		LongDescription: `
This command expects a file name or data provided via stdin.

By default it will use URL-safe base64url encoding,
but one can customize used base with -b:

  > echo hello | ipfs multibase encode -b base16 > output_file
  > cat output_file
  f68656c6c6f0a

  > echo hello > input_file
  > ipfs multibase encode -b base16 input_file
  f68656c6c6f0a
  `,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("file", true, false, "data to encode").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.StringOption(mbaseOptionName, "multibase encoding").WithDefault("base64url"),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		encoderName, _ := req.Options[mbaseOptionName].(string)
		encoder, err := mbase.EncoderByName(encoderName)
		if err != nil {
			return err
		}
		files := req.Files.Entries()
		file, err := cmdenv.GetFileArg(files)
		if err != nil {
			return fmt.Errorf("failed to access file: %w", err)
		}
		buf, err := ioutil.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read file contents: %w", err)
		}
		encoded := encoder.Encode(buf)
		reader := strings.NewReader(encoded)
		return resp.Emit(reader)
	},
}

var mbaseDecodeCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Decode multibase string",
		LongDescription: `
This command expects multibase inside of a file or via stdin:

  > echo -n hello | ipfs multibase encode -b base16 > file
  > cat file
  f68656c6c6f

  > ipfs multibase decode file
  hello

  > cat file | ipfs multibase decode
  hello
`,
	},
	Arguments: []cmds.Argument{
		cmds.FileArg("encoded_file", true, false, "encoded data to decode").EnableStdin(),
	},
	Run: func(req *cmds.Request, resp cmds.ResponseEmitter, env cmds.Environment) error {
		if err := req.ParseBodyArgs(); err != nil {
			return err
		}
		files := req.Files.Entries()
		file, err := cmdenv.GetFileArg(files)
		if err != nil {
			return fmt.Errorf("failed to access file: %w", err)
		}
		encoded_data, err := ioutil.ReadAll(file)
		if err != nil {
			return fmt.Errorf("failed to read file contents: %w", err)
		}
		_, data, err := mbase.Decode(string(encoded_data))
		if err != nil {
			return fmt.Errorf("failed to decode multibase: %w", err)
		}
		reader := bytes.NewReader(data)
		return resp.Emit(reader)
	},
}
