package main

import (
	commands "github.com/ipfs/go-ipfs/core/commands"

	cmds "github.com/ipfs/go-ipfs-cmds"
)

// This is the CLI root, used for executing commands accessible to CLI clients.
// Some subcommands (like 'ipfs daemon' or 'ipfs init') are only accessible here,
// and can't be called through the HTTP API.
var Root = &cmds.Command{
	Options:  commands.Root.Options,
	Helptext: commands.Root.Helptext,
}

// commandsClientCmd is the "ipfs commands" command for local cli
var commandsClientCmd = commands.CommandsCmd(Root)

// Commands in localCommands should always be run locally (even if daemon is running).
// They can override subcommands in commands.Root by defining a subcommand with the same name.
var localCommands = map[string]*cmds.Command{
	"daemon":   daemonCmd,
	"init":     initCmd,
	"commands": commandsClientCmd,
}

func getRoot() *cmds.Command {
	// setting here instead of in literal to prevent initialization loop
	// (some commands make references to Root)
	Root.Subcommands = localCommands

	commonCommands := commands.GetRoot()
	for k, v := range commonCommands.Subcommands {
		if _, found := Root.Subcommands[k]; !found {
			Root.Subcommands[k] = v
		}
	}

	return Root
}
