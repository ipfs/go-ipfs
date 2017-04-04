// package shell implements a remote API interface for a running ipfs daemon
package shell

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"

	cmds "github.com/ipfs/go-ipfs/commands"
	files "github.com/ipfs/go-ipfs/commands/files"
	http "github.com/ipfs/go-ipfs/commands/http"
	cc "github.com/ipfs/go-ipfs/core/commands"
)

type Shell struct {
	client http.Client
}

func NewShell(url string) *Shell {
	return &Shell{http.NewClient(url)}
}

// Cat the content at the given path
func (s *Shell) Cat(path string) (io.Reader, error) {
	ropts, err := cc.Root.GetOptions([]string{"cat"})
	if err != nil {
		return nil, err
	}

	req, err := cmds.NewRequest([]string{"cat", path}, nil, nil, nil, cc.CatCmd, ropts)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Send(req)
	if err != nil {
		return nil, err
	}

	return resp.Reader()
}

// Add a file to ipfs from the given reader, returns the hash of the added file
func (s *Shell) Add(r io.Reader) (string, error) {
	ropts, err := cc.Root.GetOptions([]string{"add"})
	if err != nil {
		return "", err
	}

	slf := files.NewSliceFile("", []files.File{files.NewReaderFile("", ioutil.NopCloser(r), nil)})

	req, err := cmds.NewRequest([]string{"add"}, nil, nil, slf, cc.AddCmd, ropts)
	if err != nil {
		return "", err
	}

	resp, err := s.client.Send(req)
	if err != nil {
		return "", err
	}
	if resp.Error() != nil {
		return "", resp.Error()
	}

	read, err := resp.Reader()
	if err != nil {
		return "", err
	}

	dec := json.NewDecoder(read)
	out := struct{ Hash string }{}
	err = dec.Decode(&out)
	if err != nil {
		return "", err
	}

	return out.Hash, nil
}

// List entries at the given path
func (s *Shell) List(path string) ([]cc.Link, error) {
	ropts, err := cc.Root.GetOptions([]string{"ls"})
	if err != nil {
		return nil, err
	}

	req, err := cmds.NewRequest([]string{"ls", path}, nil, nil, nil, cc.LsCmd, ropts)
	if err != nil {
		return nil, err
	}

	resp, err := s.client.Send(req)
	if err != nil {
		return nil, err
	}

	if resp.Error() != nil {
		return nil, resp.Error()
	}

	read, err := resp.Reader()
	if err != nil {
		return nil, err
	}

	dec := json.NewDecoder(read)
	out := struct{ Objects []cc.Object }{}
	err = dec.Decode(&out)
	if err != nil {
		return nil, err
	}

	return out.Objects[0].Links, nil
}

// Pin the given path
func (s *Shell) Pin(path string) error {
	ropts, err := cc.Root.GetOptions([]string{"pin", "add"})
	if err != nil {
		return err
	}
	pinadd := cc.PinCmd.Subcommands["add"]

	req, err := cmds.NewRequest([]string{"pin", "add", path}, nil, nil, nil, pinadd, ropts)
	if err != nil {
		return err
	}
	req.SetOption("r", true)

	resp, err := s.client.Send(req)
	if err != nil {
		return err
	}
	if resp.Error() != nil {
		return resp.Error()
	}

	read, err := resp.Reader()
	if err != nil {
		return err
	}

	out, err := ioutil.ReadAll(read)
	if err != nil {
		return err
	}

	fmt.Println(string(out))
	return nil
}
