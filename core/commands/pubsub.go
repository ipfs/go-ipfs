package commands

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"sort"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	mbase "github.com/multiformats/go-multibase"
	"github.com/pkg/errors"

	cmds "github.com/ipfs/go-ipfs-cmds"
	options "github.com/ipfs/interface-go-ipfs-core/options"
)

var PubsubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "An experimental publish-subscribe system on ipfs.",
		ShortDescription: `
ipfs pubsub allows you to publish messages to a given topic, and also to
subscribe to new messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Subcommands: map[string]*cmds.Command{
		"pub":   PubsubPubCmd,
		"sub":   PubsubSubCmd,
		"ls":    PubsubLsCmd,
		"peers": PubsubPeersCmd,
	},
}

const (
	pubsubDiscoverOptionName = "discover"
)

type pubsubMessage struct {
	From     string   `json:"from,omitempty"`
	Data     string   `json:"data,omitempty"`
	Seqno    string   `json:"seqno,omitempty"`
	TopicIDs []string `json:"topicIDs,omitempty"`
}

var PubsubSubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Subscribe to messages on a given topic.",
		ShortDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
		LongDescription: `
ipfs pubsub sub subscribes to messages on a given topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.

This command outputs data in the following encodings:
  * "json"
(Specified by the "--encoding" or "--enc" flag)
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "String name of topic to subscribe to."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(pubsubDiscoverOptionName, "Deprecated option to instruct pubsub to discovery peers for the topic. Discovery is now built into pubsub."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		topic := req.Arguments[0]
		sub, err := api.PubSub().Subscribe(req.Context, topic)
		if err != nil {
			return err
		}
		defer sub.Close()

		if f, ok := res.(http.Flusher); ok {
			f.Flush()
		}

		for {
			msg, err := sub.Next(req.Context)
			if err == io.EOF || err == context.Canceled {
				return nil
			} else if err != nil {
				return err
			}

			encoder, _ := mbase.EncoderByName("base64")
			psm := pubsubMessage{
				Data:  encoder.Encode(msg.Data()),
				From:  encoder.Encode([]byte(msg.From())),
				Seqno: encoder.Encode(msg.Seq()),
			}
			for _, topic := range msg.Topics() {
				psm.TopicIDs = append(psm.TopicIDs, encoder.Encode([]byte(topic)))
			}
			if err := res.Emit(&psm); err != nil {
				return err
			}
		}
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			_, dec, err := mbase.Decode(psm.Data)
			if err != nil {
				return err
			}
			_, err = w.Write(dec)
			return err
		}),
		"ndpayload": cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			_, dec, err := mbase.Decode(psm.Data)
			if err != nil {
				return err
			}
			data := append(dec, '\n')
			_, err = w.Write(data)
			return err
		}),
		"lenpayload": cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, psm *pubsubMessage) error {
			buf := make([]byte, 8, len(psm.Data)+8)

			n := binary.PutUvarint(buf, uint64(len(psm.Data)))
			buf = append(buf[:n], psm.Data...)
			_, err := w.Write(buf)
			return err
		}),
	},
	Type: pubsubMessage{},
}

var PubsubPubCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Publish a message to a given pubsub topic.",
		ShortDescription: `
ipfs pubsub pub publishes a message to a specified topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", true, false, "Topic to publish to."),
		cmds.StringArg("data", false, true, "Payload of message to publish."),
	},
	PreRun: func(req *cmds.Request, env cmds.Environment) error {
		encoder, _ := mbase.EncoderByName("base64")
		for n, arg := range req.Arguments {
			req.Arguments[n] = encoder.Encode([]byte(arg))
		}

		// when there are no string args, read from stdin.
		if len(req.Arguments) == 1 {
			buf, err := ioutil.ReadAll(os.Stdin)
			if err != nil {
				return err
			}
			req.Arguments = append(req.Arguments, encoder.Encode(buf))
		}
		return nil
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		_, topic, err := mbase.Decode(req.Arguments[0])
		if err != nil {
			return errors.Wrap(err, "pubsub topic must be multibase encoded")
		}

		for _, data := range req.Arguments[1:] {
			_, datab, err := mbase.Decode(data)
			if err != nil {
				return errors.Wrap(err, "pubsub data must be multibase encoded")
			}
			if err := api.PubSub().Publish(req.Context, string(topic), datab); err != nil {
				return err
			}
		}

		return nil
	},
}

var PubsubLsCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List subscribed topics by name.",
		ShortDescription: `
ipfs pubsub ls lists out the names of topics you are currently subscribed to.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		l, err := api.PubSub().Ls(req.Context)
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, stringList{l})
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(stringListEncoder),
	},
}

func stringListEncoder(req *cmds.Request, w io.Writer, list *stringList) error {
	for _, str := range list.Strings {
		_, err := fmt.Fprintf(w, "%s\n", cmdenv.EscNonPrint(str))
		if err != nil {
			return err
		}
	}
	return nil
}

var PubsubPeersCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "List peers we are currently pubsubbing with.",
		ShortDescription: `
ipfs pubsub peers with no arguments lists out the pubsub peers you are
currently connected to. If given a topic, it will list connected
peers who are subscribed to the named topic.

This is an experimental feature. It is not intended in its current state
to be used in a production environment.

To use, the daemon must be run with '--enable-pubsub-experiment'.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("topic", false, false, "topic to list connected peers of"),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		api, err := cmdenv.GetApi(env, req)
		if err != nil {
			return err
		}

		var topic string
		if len(req.Arguments) == 1 {
			topic = req.Arguments[0]
		}

		peers, err := api.PubSub().Peers(req.Context, options.PubSub.Topic(topic))
		if err != nil {
			return err
		}

		list := &stringList{make([]string, 0, len(peers))}

		for _, peer := range peers {
			list.Strings = append(list.Strings, peer.Pretty())
		}
		sort.Strings(list.Strings)
		return cmds.EmitOnce(res, list)
	},
	Type: stringList{},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(stringListEncoder),
	},
}
