package commands

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"time"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	dag "github.com/ipfs/go-ipfs/merkledag"
	path "github.com/ipfs/go-ipfs/path"

	routing "gx/ipfs/QmPCGUjMRuBcPybZFpjhzpifwPP9wPRoiy5geTQKU4vqWA/go-libp2p-routing"
	notif "gx/ipfs/QmPCGUjMRuBcPybZFpjhzpifwPP9wPRoiy5geTQKU4vqWA/go-libp2p-routing/notifications"
	"gx/ipfs/QmQp2a2Hhb7F6eK2A5hN8f9aJy4mtkEikL9Zj4cgB7d1dD/go-ipfs-cmdkit"
	b58 "gx/ipfs/QmT8rehPR3F6bmwL6zjUN8XpiDBFFpMP2myPdC6ApsWfJf/go-base58"
	peer "gx/ipfs/QmWNY7dV54ZDYmTA1ykVdwNCqC11mpU4zSUp6XDpLTH9eG/go-libp2p-peer"
	pstore "gx/ipfs/QmYijbtjCxFEjSXaudaQAUz3LN5VKLssm8WCUsRoqzXmQR/go-libp2p-peerstore"
	cid "gx/ipfs/QmeSrf6pzut73u6zLQkRFQ3ygt3k6XFT2kjdYP8Tnkwwyg/go-cid"
	ipdht "gx/ipfs/QmfUvYQhL2GinafMbPDYz7VFoZv4iiuLuR33aRsPurXGag/go-libp2p-kad-dht"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

var DhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Issue commands directly through the DHT.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"query":     queryDhtCmd,
		"findprovs": findProvidersDhtCmd,
		"findpeer":  findPeerDhtCmd,
		"get":       getValueDhtCmd,
		"put":       putValueDhtCmd,
		"provide":   provideRefDhtCmd,
	},
}

var queryDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, true, "The peerID to run the query against."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		k := string(b58.Decode(req.Arguments()[0]))

		closestPeers, err := dht.GetClosestPeers(ctx, k)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		go func() {
			defer close(events)
			for p := range closestPeers {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					ID:   p,
					Type: notif.FinalPeer,
				})
			}
		}()

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() cmds.Marshaler {
			pfm := pfuncMap{
				notif.PeerResponse: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					for _, p := range obj.Responses {
						fmt.Fprintf(out, "%s\n", p.ID.Pretty())
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				verbose, _, _ := res.Request().Option("v").Bool()

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Find peers in the DHT that can provide a specific value, given a key.",
		ShortDescription: "Outputs a list of newline-delimited provider Peer IDs.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key to find providers for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
		cmdkit.IntOption("num-providers", "n", "The number of providers to find.").WithDefault(20),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		numProviders, _, err := res.Request().Option("num-providers").Int()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}
		if numProviders < 1 {
			res.SetError(fmt.Errorf("Number of providers must be greater than 0"), cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		c, err := cid.Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		pchan := dht.FindProvidersAsync(ctx, c, numProviders)
		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer close(events)
			for p := range pchan {
				np := p
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:      notif.Provider,
					Responses: []*pstore.PeerInfo{&np},
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Provider: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					prov := obj.Responses[0]
					if verbose {
						fmt.Fprintf(out, "provider: ")
					}
					fmt.Fprintf(out, "%s\n", prov.ID.Pretty())
					if verbose {
						for _, a := range prov.Addrs {
							fmt.Fprintf(out, "\t%s\n", a)
						}
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option("v").Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var provideRefDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Announce to the network that you are providing given values.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key[s] to send provide records for.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
		cmdkit.BoolOption("recursive", "r", "Recursively provide entire graph."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if n.Routing == nil {
			res.SetError(errNotOnline, cmdkit.ErrNormal)
			return
		}

		if len(n.PeerHost.Network().Conns()) == 0 {
			res.SetError(errors.New("cannot provide, no connected peers"), cmdkit.ErrNormal)
			return
		}

		rec, _, _ := req.Option("recursive").Bool()

		var cids []*cid.Cid
		for _, arg := range req.Arguments() {
			c, err := cid.Decode(arg)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			has, err := n.Blockstore.Has(c)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}

			if !has {
				res.SetError(fmt.Errorf("block %s not found locally, cannot provide", c), cmdkit.ErrNormal)
				return
			}

			cids = append(cids, c)
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer close(events)
			var err error
			if rec {
				err = provideKeysRec(ctx, n.Routing, n.DAG, cids)
			} else {
				err = provideKeys(ctx, n.Routing, cids)
			}
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(res cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "sending provider record to peer %s\n", obj.ID)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option("v").Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)
				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

func provideKeys(ctx context.Context, r routing.IpfsRouting, cids []*cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.IpfsRouting, dserv dag.DAGService, cids []*cid.Cid) error {
	provided := cid.NewSet()
	for _, c := range cids {
		kset := cid.NewSet()
		visitor := dag.FetchingVisitor(ctx, kset, dserv)

		err := dag.EnumerateChildrenAsync(ctx, dserv.GetLinks, c, visitor)
		if err != nil {
			return err
		}

		for _, k := range kset.Keys() {
			if provided.Has(k) {
				continue
			}

			err = r.Provide(ctx, k, true)
			if err != nil {
				return err
			}
			provided.Add(k)
		}
	}

	return nil
}

var findPeerDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline:          "Query the DHT for all of the multiaddresses associated with a Peer ID.",
		ShortDescription: "Outputs a list of newline-delimited multiaddresses.",
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("peerID", true, true, "The ID of the peer to search for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		pid, err := peer.IDB58Decode(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		go func() {
			defer close(outChan)
			for v := range events {
				select {
				case outChan <- v:
				case <-req.Context().Done():
				}

			}
		}()

		go func() {
			defer close(events)
			pi, err := dht.FindPeer(ctx, pid)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
				return
			}

			notif.PublishQueryEvent(ctx, &notif.QueryEvent{
				Type:      notif.FinalPeer,
				Responses: []*pstore.PeerInfo{&pi},
			})
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					pi := obj.Responses[0]
					for _, a := range pi.Addrs {
						fmt.Fprintf(out, "%s\n", a)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option("v").Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var getValueDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Given a key, query the DHT for its best value.",
		ShortDescription: `
Outputs the best value for the given key.

There may be several different values for a given key stored in the DHT; in
this context 'best' means the record that is most desirable. There is no one
metric for 'best': it depends entirely on the key type. For IPNS, 'best' is
the record that is both valid and has the highest sequence number (freshest).
Different key types can specify other 'best' rules.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, true, "The key to find a value for."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		dhtkey, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
				}
			}
		}()

		go func() {
			defer close(events)
			val, err := dht.GetValue(ctx, dhtkey)
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			} else {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.Value,
					Extra: string(val),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
					} else {
						fmt.Fprintln(out, obj.Extra)
					}
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option("v").Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}

				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)

				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

var putValueDhtCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Write a key/value pair to the DHT.",
		ShortDescription: `
Given a key of the form /foo/bar and a value of any form, this will write that
value to the DHT with that key.

Keys have two parts: a keytype (foo) and the key name (bar). IPNS uses the
/ipns keytype, and expects the key name to be a Peer ID. IPNS entries are
specifically formatted (protocol buffer).

You may only use keytypes that are supported in your ipfs binary: currently
this is only /ipns. Unless you have a relatively deep understanding of the
go-ipfs DHT internals, you likely want to be using 'ipfs name publish' instead
of this.

Value is arbitrary text. Standard input can be used to provide value.

NOTE: A value may not exceed 2048 bytes.
`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("key", true, false, "The key to store the value at."),
		cmdkit.StringArg("value", true, false, "The value to store.").EnableStdin(),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption("verbose", "v", "Print extra information."),
	},
	Run: func(req cmds.Request, res cmds.Response) {
		n, err := req.InvocContext().GetNode()
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		dht, ok := n.Routing.(*ipdht.IpfsDHT)
		if !ok {
			res.SetError(ErrNotDHT, cmdkit.ErrNormal)
			return
		}

		outChan := make(chan interface{})
		res.SetOutput((<-chan interface{})(outChan))

		events := make(chan *notif.QueryEvent)
		ctx := notif.RegisterForQueryEvents(req.Context(), events)

		key, err := escapeDhtKey(req.Arguments()[0])
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		data := req.Arguments()[1]

		go func() {
			defer close(outChan)
			for e := range events {
				select {
				case outChan <- e:
				case <-req.Context().Done():
					return
				}
			}
		}()

		go func() {
			defer close(events)
			err := dht.PutValue(ctx, key, []byte(data))
			if err != nil {
				notif.PublishQueryEvent(ctx, &notif.QueryEvent{
					Type:  notif.QueryError,
					Extra: err.Error(),
				})
			}
		}()
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func() func(cmds.Response) (io.Reader, error) {
			pfm := pfuncMap{
				notif.FinalPeer: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
				},
				notif.Value: func(obj *notif.QueryEvent, out io.Writer, verbose bool) {
					fmt.Fprintf(out, "%s\n", obj.ID.Pretty())
				},
			}

			return func(res cmds.Response) (io.Reader, error) {
				verbose, _, _ := res.Request().Option("v").Bool()
				v, err := unwrapOutput(res.Output())
				if err != nil {
					return nil, err
				}
				obj, ok := v.(*notif.QueryEvent)
				if !ok {
					return nil, e.TypeErr(obj, v)
				}

				buf := new(bytes.Buffer)
				printEvent(obj, buf, verbose, pfm)

				return buf, nil
			}
		}(),
	},
	Type: notif.QueryEvent{},
}

type printFunc func(obj *notif.QueryEvent, out io.Writer, verbose bool)
type pfuncMap map[notif.QueryEventType]printFunc

func printEvent(obj *notif.QueryEvent, out io.Writer, verbose bool, override pfuncMap) {
	if verbose {
		fmt.Fprintf(out, "%s: ", time.Now().Format("15:04:05.000"))
	}

	if override != nil {
		if pf, ok := override[obj.Type]; ok {
			pf(obj, out, verbose)
			return
		}
	}

	switch obj.Type {
	case notif.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID)
		}
	case notif.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
		} else {
			fmt.Fprint(out, obj.Extra)
		}
	case notif.PeerResponse:
		if verbose {
			fmt.Fprintf(out, "* %s says use ", obj.ID)
			for _, p := range obj.Responses {
				fmt.Fprintf(out, "%s ", p.ID)
			}
			fmt.Fprintln(out)
		}
	case notif.QueryError:
		if verbose {
			fmt.Fprintf(out, "error: %s\n", obj.Extra)
		}
	case notif.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID)
		}
	case notif.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID)
		}
	case notif.FinalPeer:
	default:
		if verbose {
			fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type)
		}
	}
}

func escapeDhtKey(s string) (string, error) {
	parts := path.SplitList(s)
	switch len(parts) {
	case 1:
		return string(b58.Decode(s)), nil
	case 3:
		k := b58.Decode(parts[2])
		return path.Join(append(parts[:2], string(k))), nil
	default:
		return "", errors.New("invalid key")
	}
}
