package commands

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"

	cid "github.com/ipfs/go-cid"
	cmds "github.com/ipfs/go-ipfs-cmds"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	path "github.com/ipfs/go-path"
	peer "github.com/libp2p/go-libp2p-core/peer"
	routing "github.com/libp2p/go-libp2p-core/routing"
	b58 "github.com/mr-tron/base58/base58"
)

var ErrNotDHT = errors.New("routing service is not a DHT")

// TODO: Factor into `ipfs dht` and `ipfs routing`.
// Everything *except `query` goes into `ipfs routing`.

var DhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
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

const (
	dhtVerboseOptionName = "verbose"
)

var queryDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Find the closest Peer IDs to a given Peer ID by querying the DHT.",
		ShortDescription: "Outputs a list of newline-delimited Peer IDs.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The peerID to run the query against."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if nd.DHT == nil {
			return ErrNotDHT
		}

		id, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return cmds.ClientError("invalid peer ID")
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		dht := nd.DHT.WAN
		if !nd.DHT.WANActive() {
			dht = nd.DHT.LAN
		}

		errCh := make(chan error, 1)
		go func() {
			defer close(errCh)
			defer cancel()
			closestPeers, err := dht.GetClosestPeers(ctx, string(id))
			if closestPeers != nil {
				for p := range closestPeers {
					routing.PublishQueryEvent(ctx, &routing.QueryEvent{
						ID:   p,
						Type: routing.FinalPeer,
					})
				}
			}

			if err != nil {
				errCh <- err
				return
			}
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return <-errCh
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					fmt.Fprintf(out, "%s\n", obj.ID)
					return nil
				},
			}
			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

const (
	numProvidersOptionName = "num-providers"
)

var findProvidersDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Find peers that can provide a specific value, given a key.",
		ShortDescription: "Outputs a list of newline-delimited provider Peer IDs.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key to find providers for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
		cmds.IntOption(numProvidersOptionName, "n", "The number of providers to find.").WithDefault(20),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !n.IsOnline {
			return ErrNotOnline
		}

		numProviders, _ := req.Options[numProvidersOptionName].(int)
		if numProviders < 1 {
			return fmt.Errorf("number of providers must be greater than 0")
		}

		c, err := cid.Parse(req.Arguments[0])

		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		pchan := n.Routing.FindProvidersAsync(ctx, c, numProviders)

		go func() {
			defer cancel()
			for p := range pchan {
				np := p
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:      routing.Provider,
					Responses: []*peer.AddrInfo{&np},
				})
			}
		}()
		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return nil
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
					return nil
				},
				routing.Provider: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
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
					return nil
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

const (
	recursiveOptionName = "recursive"
)

var provideRefDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Announce to the network that you are providing given values.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key[s] to send provide records for.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
		cmds.BoolOption(recursiveOptionName, "r", "Recursively provide entire graph."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		if len(nd.PeerHost.Network().Conns()) == 0 {
			return errors.New("cannot provide, no connected peers")
		}

		// Needed to parse stdin args.
		// TODO: Lazy Load
		err = req.ParseBodyArgs()
		if err != nil {
			return err
		}

		rec, _ := req.Options[recursiveOptionName].(bool)

		var cids []cid.Cid
		for _, arg := range req.Arguments {
			c, err := cid.Decode(arg)
			if err != nil {
				return err
			}

			has, err := nd.Blockstore.Has(req.Context, c)
			if err != nil {
				return err
			}

			if !has {
				return fmt.Errorf("block %s not found locally, cannot provide", c)
			}

			cids = append(cids, c)
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		var provideErr error
		go func() {
			defer cancel()
			if rec {
				provideErr = provideKeysRec(ctx, nd.Routing, nd.DAG, cids)
			} else {
				provideErr = provideKeys(ctx, nd.Routing, cids)
			}
			if provideErr != nil {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:  routing.QueryError,
					Extra: provideErr.Error(),
				})
			}
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return provideErr
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					if verbose {
						fmt.Fprintf(out, "sending provider record to peer %s\n", obj.ID)
					}
					return nil
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

func provideKeys(ctx context.Context, r routing.Routing, cids []cid.Cid) error {
	for _, c := range cids {
		err := r.Provide(ctx, c, true)
		if err != nil {
			return err
		}
	}
	return nil
}

func provideKeysRec(ctx context.Context, r routing.Routing, dserv ipld.DAGService, cids []cid.Cid) error {
	provided := cid.NewSet()
	for _, c := range cids {
		kset := cid.NewSet()

		err := dag.Walk(ctx, dag.GetLinksDirect(dserv), c, kset.Visit)
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
	Helptext: cmds.HelpText{
		Tagline:          "Find the multiaddresses associated with a Peer ID.",
		ShortDescription: "Outputs a list of newline-delimited multiaddresses.",
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("peerID", true, true, "The ID of the peer to search for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		pid, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		var findPeerErr error
		go func() {
			defer cancel()
			var pi peer.AddrInfo
			pi, findPeerErr = nd.Routing.FindPeer(ctx, pid)
			if findPeerErr != nil {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:  routing.QueryError,
					Extra: findPeerErr.Error(),
				})
				return
			}

			routing.PublishQueryEvent(ctx, &routing.QueryEvent{
				Type:      routing.FinalPeer,
				Responses: []*peer.AddrInfo{&pi},
			})
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return findPeerErr
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					pi := obj.Responses[0]
					for _, a := range pi.Addrs {
						fmt.Fprintf(out, "%s\n", a)
					}
					return nil
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

var getValueDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Given a key, query the routing system for its best value.",
		ShortDescription: `
Outputs the best value for the given key.

There may be several different values for a given key stored in the routing
system; in this context 'best' means the record that is most desirable. There is
no one metric for 'best': it depends entirely on the key type. For IPNS, 'best'
is the record that is both valid and has the highest sequence number (freshest).
Different key types can specify other 'best' rules.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, true, "The key to find a value for."),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		dhtkey, err := escapeDhtKey(req.Arguments[0])
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		var getErr error
		go func() {
			defer cancel()
			var val []byte
			val, getErr = nd.Routing.GetValue(ctx, dhtkey)
			if getErr != nil {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:  routing.QueryError,
					Extra: getErr.Error(),
				})
			} else {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:  routing.Value,
					Extra: base64.StdEncoding.EncodeToString(val),
				})
			}
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return getErr
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.Value: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					if verbose {
						_, err := fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
						return err
					}
					res, err := base64.StdEncoding.DecodeString(obj.Extra)
					if err != nil {
						return err
					}
					_, err = out.Write(res)
					return err
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)
			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

var putValueDhtCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Write a key/value pair to the routing system.",
		ShortDescription: `
Given a key of the form /foo/bar and a valid value for that key, this will write
that value to the routing system with that key.

Keys have two parts: a keytype (foo) and the key name (bar). IPNS uses the
/ipns keytype, and expects the key name to be a Peer ID. IPNS entries are
specifically formatted (protocol buffer).

You may only use keytypes that are supported in your ipfs binary: currently
this is only /ipns. Unless you have a relatively deep understanding of the
go-ipfs routing internals, you likely want to be using 'ipfs name publish' instead
of this.

The value must be a valid value for the given key type. For example, if the key
is /ipns/QmFoo, the value must be IPNS record (protobuf) signed with the key
identified by QmFoo.
`,
	},

	Arguments: []cmds.Argument{
		cmds.StringArg("key", true, false, "The key to store the value at."),
		cmds.FileArg("value-file", true, false, "A path to a file containing the value to store.").EnableStdin(),
	},
	Options: []cmds.Option{
		cmds.BoolOption(dhtVerboseOptionName, "v", "Print extra information."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		key, err := escapeDhtKey(req.Arguments[0])
		if err != nil {
			return err
		}

		file, err := cmdenv.GetFileArg(req.Files.Entries())
		if err != nil {
			return err
		}
		defer file.Close()

		data, err := ioutil.ReadAll(file)
		if err != nil {
			return err
		}

		ctx, cancel := context.WithCancel(req.Context)
		ctx, events := routing.RegisterForQueryEvents(ctx)

		var putErr error
		go func() {
			defer cancel()
			putErr = nd.Routing.PutValue(ctx, key, []byte(data))
			if putErr != nil {
				routing.PublishQueryEvent(ctx, &routing.QueryEvent{
					Type:  routing.QueryError,
					Extra: putErr.Error(),
				})
			}
		}()

		for e := range events {
			if err := res.Emit(e); err != nil {
				return err
			}
		}

		return putErr
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *routing.QueryEvent) error {
			pfm := pfuncMap{
				routing.FinalPeer: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					if verbose {
						fmt.Fprintf(out, "* closest peer %s\n", obj.ID)
					}
					return nil
				},
				routing.Value: func(obj *routing.QueryEvent, out io.Writer, verbose bool) error {
					fmt.Fprintf(out, "%s\n", obj.ID.Pretty())
					return nil
				},
			}

			verbose, _ := req.Options[dhtVerboseOptionName].(bool)

			return printEvent(out, w, verbose, pfm)
		}),
	},
	Type: routing.QueryEvent{},
}

type printFunc func(obj *routing.QueryEvent, out io.Writer, verbose bool) error
type pfuncMap map[routing.QueryEventType]printFunc

func printEvent(obj *routing.QueryEvent, out io.Writer, verbose bool, override pfuncMap) error {
	if verbose {
		fmt.Fprintf(out, "%s: ", time.Now().Format("15:04:05.000"))
	}

	if override != nil {
		if pf, ok := override[obj.Type]; ok {
			return pf(obj, out, verbose)
		}
	}

	switch obj.Type {
	case routing.SendingQuery:
		if verbose {
			fmt.Fprintf(out, "* querying %s\n", obj.ID)
		}
	case routing.Value:
		if verbose {
			fmt.Fprintf(out, "got value: '%s'\n", obj.Extra)
		} else {
			fmt.Fprint(out, obj.Extra)
		}
	case routing.PeerResponse:
		if verbose {
			fmt.Fprintf(out, "* %s says use ", obj.ID)
			for _, p := range obj.Responses {
				fmt.Fprintf(out, "%s ", p.ID)
			}
			fmt.Fprintln(out)
		}
	case routing.QueryError:
		if verbose {
			fmt.Fprintf(out, "error: %s\n", obj.Extra)
		}
	case routing.DialingPeer:
		if verbose {
			fmt.Fprintf(out, "dialing peer: %s\n", obj.ID)
		}
	case routing.AddingPeer:
		if verbose {
			fmt.Fprintf(out, "adding peer to query: %s\n", obj.ID)
		}
	case routing.FinalPeer:
	default:
		if verbose {
			fmt.Fprintf(out, "unrecognized event type: %d\n", obj.Type)
		}
	}
	return nil
}

func escapeDhtKey(s string) (string, error) {
	parts := path.SplitList(s)
	switch len(parts) {
	case 1:
		k, err := b58.Decode(s)
		if err != nil {
			return "", err
		}
		return string(k), nil
	case 3:
		k, err := b58.Decode(parts[2])
		if err != nil {
			return "", err
		}
		return path.Join(append(parts[:2], string(k))), nil
	default:
		return "", errors.New("invalid key")
	}
}
