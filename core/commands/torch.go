package commands

import (
	"fmt"
	"io"
	"strings"

	context "gx/ipfs/QmZy2y8t9zQH2a1b8q2ZSLKp17ATuJoCNxxyMFG5qFExpt/go-net/context"

	cmds "github.com/ipfs/go-ipfs/commands"

	"gx/ipfs/QmRBqJF7hb8ZSpRcMwUt8hNhydWcxGEhtk81HKq6oUwKvs/go-libp2p-peer"
)

var TorchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "a one to many pubsub",
	},
	Subcommands: map[string]*cmds.Command{
		"pub":    torchPublishCmd,
		"watch":  torchWatchCmd,
		"create": torchCreateCmd,
		"rm":     torchRmCmd,
	},
}

var torchCreateCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "create a new torch topic",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "topic name"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		n.PubSub.NewTopic(context.Background(), req.Arguments()[0])
	},
}

var torchRmCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "remove a torch topic",
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "topic name to delete"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		tname := req.Arguments()[0]

		t, ok := n.PubSub.Topics[tname]
		if !ok {
			res.SetError(fmt.Errorf("no such topic '%s'", tname), cmds.ErrNormal)
			return
		}

		err = t.Close()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		delete(n.PubSub.Topics, tname)
	},
}

var torchPublishCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "publish some content to watchers",
	},
	Options: []cmds.Option{
		cmds.StringOption("topic", "t", "topic to publish to"),
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("data", true, false, "thing to publish"),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		topic, found, _ := req.Option("topic").String()
		if !found {
			res.SetError(fmt.Errorf("no topic specified (use -t)"), cmds.ErrNormal)
			return
		}

		t, ok := n.PubSub.Topics[topic]
		if !ok {
			res.SetError(fmt.Errorf("no such topic %s", topic), cmds.ErrNormal)
			return
		}

		err = t.PublishMessage([]byte(req.Arguments()[0]))
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}
	},
}

var torchWatchCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "watch for content from a given publisher",
		ShortDescription: `
'ipfs torch watch' watches for content from a given publisher/topic.
Any content they publish to the selected topic will be output.

Example:

   $ ipfs torch watch Qmajxqssk9RtjifqS5NpAvdVT8iPvSxb4zDDRPCYQUUe8E/time
   10:32:54
   10:32:55
   10:32:56
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topicID", true, false, ""),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		ctx := req.Context()
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			res.SetError(errNotOnline, cmds.ErrClient)
			return
		}

		id := req.Arguments()[0]
		parts := strings.SplitN(id, "/", 2)
		pid, err := peer.IDB58Decode(parts[0])
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		topic := parts[1]

		sub, err := n.PubSub.Subscribe(ctx, pid, topic)
		if err != nil {
			res.SetError(err, cmds.ErrNormal)
			return
		}

		pr, pw := io.Pipe()
		go func() {
			defer sub.Close()
			defer pw.Close()
			for val := range sub.Messages() {
				pw.Write(val)
			}
		}()
		res.SetOutput(pr)
	},
}
