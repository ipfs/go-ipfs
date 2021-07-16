package commands

import (
	"fmt"
	"io"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"

	humanize "github.com/dustin/go-humanize"
	bitswap "github.com/ipfs/go-bitswap"
	decision "github.com/ipfs/go-bitswap/decision"
	cidutil "github.com/ipfs/go-cidutil"
	cmds "github.com/ipfs/go-ipfs-cmds"
	peer "github.com/libp2p/go-libp2p-core/peer"
)

var BitswapCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Interact with the bitswap agent.",
		ShortDescription: ``,
	},

	Subcommands: map[string]*cmds.Command{
		"stat":      bitswapStatCmd,
		"wantlist":  showWantlistCmd,
		"ledger":    ledgerCmd,
		"reprovide": reprovideCmd,
	},
}

const (
	peerOptionName = "peer"
)

var showWantlistCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show blocks currently on the wantlist.",
		ShortDescription: `
Print out all blocks currently on the bitswap wantlist for the local peer.`,
	},
	Options: []cmds.Option{
		cmds.StringOption(peerOptionName, "p", "Specify which peer to show wantlist for. Default: self."),
	},
	Type: KeyList{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		pstr, found := req.Options[peerOptionName].(string)
		if found {
			pid, err := peer.Decode(pstr)
			if err != nil {
				return err
			}
			if pid != nd.Identity {
				return cmds.EmitOnce(res, &KeyList{bs.WantlistForPeer(pid)})
			}
		}

		return cmds.EmitOnce(res, &KeyList{bs.GetWantlist()})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *KeyList) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			// sort the keys first
			cidutil.Sort(out.Keys)
			for _, key := range out.Keys {
				fmt.Fprintln(w, enc.Encode(key))
			}
			return nil
		}),
	},
}

const (
	bitswapVerboseOptionName = "verbose"
	bitswapHumanOptionName   = "human"
)

var bitswapStatCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:          "Show some diagnostic information on the bitswap agent.",
		ShortDescription: ``,
	},
	Options: []cmds.Option{
		cmds.BoolOption(bitswapVerboseOptionName, "v", "Print extra information"),
		cmds.BoolOption(bitswapHumanOptionName, "Print sizes in human readable format (e.g., 1K 234M 2G)"),
	},
	Type: bitswap.Stat{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return cmds.Errorf(cmds.ErrClient, ErrNotOnline.Error())
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		st, err := bs.Stat()
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, st)
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, s *bitswap.Stat) error {
			enc, err := cmdenv.GetLowLevelCidEncoder(req)
			if err != nil {
				return err
			}
			verbose, _ := req.Options[bitswapVerboseOptionName].(bool)
			human, _ := req.Options[bitswapHumanOptionName].(bool)

			fmt.Fprintln(w, "bitswap status")
			fmt.Fprintf(w, "\tprovides buffer: %d / %d\n", s.ProvideBufLen, bitswap.HasBlockBufferSize)
			fmt.Fprintf(w, "\tblocks received: %d\n", s.BlocksReceived)
			fmt.Fprintf(w, "\tblocks sent: %d\n", s.BlocksSent)
			if human {
				fmt.Fprintf(w, "\tdata received: %s\n", humanize.Bytes(s.DataReceived))
				fmt.Fprintf(w, "\tdata sent: %s\n", humanize.Bytes(s.DataSent))
			} else {
				fmt.Fprintf(w, "\tdata received: %d\n", s.DataReceived)
				fmt.Fprintf(w, "\tdata sent: %d\n", s.DataSent)
			}
			fmt.Fprintf(w, "\tdup blocks received: %d\n", s.DupBlksReceived)
			if human {
				fmt.Fprintf(w, "\tdup data received: %s\n", humanize.Bytes(s.DupDataReceived))
			} else {
				fmt.Fprintf(w, "\tdup data received: %d\n", s.DupDataReceived)
			}
			fmt.Fprintf(w, "\twantlist [%d keys]\n", len(s.Wantlist))
			for _, k := range s.Wantlist {
				fmt.Fprintf(w, "\t\t%s\n", enc.Encode(k))
			}

			fmt.Fprintf(w, "\tinboundreqs [%d]\n", s.QueuedRequests)

			fmt.Fprintf(w, "\tpartners [%d]\n", len(s.Peers))
			if verbose {
				for _, p := range s.Peers {
					fmt.Fprintf(w, "\t\t%s\n", p)
				}
			}

			return nil
		}),
	},
}

var ledgerCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Show the current ledger for a peer.",
		ShortDescription: `
The Bitswap decision engine tracks the number of bytes exchanged between IPFS
nodes, and stores this information as a collection of ledgers. This command
prints the ledger associated with a given peer.
`,
	},
	Arguments: []cmds.Argument{
		cmds.StringArg("peer", true, false, "The PeerID (B58) of the ledger to inspect."),
	},
	Type: decision.Receipt{},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		bs, ok := nd.Exchange.(*bitswap.Bitswap)
		if !ok {
			return e.TypeErr(bs, nd.Exchange)
		}

		partner, err := peer.Decode(req.Arguments[0])
		if err != nil {
			return err
		}

		return cmds.EmitOnce(res, bs.LedgerForPeer(partner))
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeTypedEncoder(func(req *cmds.Request, w io.Writer, out *decision.Receipt) error {
			fmt.Fprintf(w, "Ledger for %s\n"+
				"Debt ratio:\t%f\n"+
				"Exchanges:\t%d\n"+
				"Bytes sent:\t%d\n"+
				"Bytes received:\t%d\n\n",
				out.Peer, out.Value, out.Exchanged,
				out.Sent, out.Recv)
			return nil
		}),
	},
}

var reprovideCmd = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline: "Trigger reprovider.",
		ShortDescription: `
Trigger reprovider to announce our data to network.
`,
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) error {
		nd, err := cmdenv.GetNode(env)
		if err != nil {
			return err
		}

		if !nd.IsOnline {
			return ErrNotOnline
		}

		err = nd.Provider.Reprovide(req.Context)
		if err != nil {
			return err
		}

		return nil
	},
}
