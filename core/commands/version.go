package commands

import (
	"fmt"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	config "github.com/ipfs/go-ipfs/repo/config"
)

type VersionOutput struct {
	Version string
	Commit string
}

var VersionCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Shows ipfs version information",
		ShortDescription: "Returns the current version of ipfs and exits.",
	},

	Options: []cmds.Option{
		cmds.BoolOption("number", 'n', "Only show the version number"),
		cmds.BoolOption("commit", 0, "Show the commit hash"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		res.SetOutput(&VersionOutput{
			Version: config.CurrentVersionNumber,
			Commit: config.CurrentCommit,
		})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v := res.Output().(*VersionOutput)

			commit, found, err := res.Request().Option("commit").Bool()
			commitTxt := ""
			if err != nil {
				return nil, err
			}
			if found && commit {
				commitTxt = "-" + v.Commit
			}

			number, found, err := res.Request().Option("number").Bool()
			if err != nil {
				return nil, err
			}
			if found && number {
				return strings.NewReader(fmt.Sprintln(v.Version + commitTxt)), nil
			}

			return strings.NewReader(fmt.Sprintf("ipfs version %s%s\n", v.Version, commitTxt)), nil
		},
	},
	Type: VersionOutput{},
}
