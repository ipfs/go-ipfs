package commands

import (
	"fmt"

	cmds "github.com/jbenet/go-ipfs/commands"
	"github.com/jbenet/go-ipfs/core"
	"github.com/jbenet/go-ipfs/core/commands2/internal"
	"github.com/jbenet/go-ipfs/merkledag"
)

var pinCmd = &cmds.Command{
	Description: "Keeps objects stored locally",

	Subcommands: map[string]*cmds.Command{
		"add": addPinCmd,
		"rm":  rmPinCmd,
	},
}

var addPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Pins objects to local storage",
		ShortDescription: `
Retrieves the object named by <ipfs-path> and stores it locally
on disk.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be pinned"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively pin the object linked to by the specified object(s)"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// set recursive flag
		recursive, found, err := req.Option("recursive").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			recursive = false
		}

		paths, err := internal.CastToStrings(req.Arguments())
		if err != nil {
			return nil, err
		}

		_, err = pin(n, paths, recursive)
		if err != nil {
			return nil, err
		}

		// TODO: create some output to show what got pinned
		return nil, nil
	},
}

var rmPinCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Unpin an object from local storage",
		ShortDescription: `
Removes the pin from the given object allowing it to be garbage
collected if needed.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("ipfs-path", true, true, "Path to object(s) to be unpinned"),
	},
	Options: []cmds.Option{
		cmds.BoolOption("recursive", "r", "Recursively unpin the object linked to by the specified object(s)"),
	},
	Run: func(req cmds.Request) (interface{}, error) {
		n, err := req.Context().GetNode()
		if err != nil {
			return nil, err
		}

		// set recursive flag
		recursive, found, err := req.Option("recursive").Bool()
		if err != nil {
			return nil, err
		}
		if !found {
			recursive = false // default
		}

		paths, err := internal.CastToStrings(req.Arguments())
		if err != nil {
			return nil, err
		}

		_, err = unpin(n, paths, recursive)
		if err != nil {
			return nil, err
		}

		// TODO: create some output to show what got unpinned
		return nil, nil
	},
}

func pin(n *core.IpfsNode, paths []string, recursive bool) ([]*merkledag.Node, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, path := range paths {
		dagnode, err := n.Resolver.ResolvePath(path)
		if err != nil {
			return nil, fmt.Errorf("pin error: %v", err)
		}
		dagnodes = append(dagnodes, dagnode)
	}

	for _, dagnode := range dagnodes {
		err := n.Pinning.Pin(dagnode, recursive)
		if err != nil {
			return nil, fmt.Errorf("pin: %v", err)
		}
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}

	return dagnodes, nil
}

func unpin(n *core.IpfsNode, paths []string, recursive bool) ([]*merkledag.Node, error) {

	dagnodes := make([]*merkledag.Node, 0)
	for _, path := range paths {
		dagnode, err := n.Resolver.ResolvePath(path)
		if err != nil {
			return nil, err
		}
		dagnodes = append(dagnodes, dagnode)
	}

	for _, dagnode := range dagnodes {
		k, _ := dagnode.Key()
		err := n.Pinning.Unpin(k, recursive)
		if err != nil {
			return nil, err
		}
	}

	err := n.Pinning.Flush()
	if err != nil {
		return nil, err
	}
	return dagnodes, nil
}
