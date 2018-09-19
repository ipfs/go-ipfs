package name

import (
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	"github.com/ipfs/go-ipfs/keystore"
	namesys "github.com/ipfs/go-ipfs/namesys"
	nsopts "github.com/ipfs/go-ipfs/namesys/opts"

	cmds "gx/ipfs/QmPTfgFTo9PFr1PvPKyKoeMgBvYPh6cX3aDP7DHKVbnCbi/go-ipfs-cmds"
	"gx/ipfs/QmQsErDt8Qgw1XrsXf2BpEzDgGWtB1YLsTAARBup5b6B9W/go-libp2p-peer"
	logging "gx/ipfs/QmRREK2CAZ5Re2Bd9zZFG6FeYDppUWt5cMgsoUEp3ktgSr/go-log"
	offline "gx/ipfs/QmSNe4MWVxZWk6UxxW2z2EKofFo4GdFzud1vfn1iVby3mj/go-ipfs-routing/offline"
	"gx/ipfs/QmSP88ryZkHSRn1fnngAaV2Vcn63WUJzAavnRM9CVdU1Ky/go-ipfs-cmdkit"
	path "gx/ipfs/QmX7uSbkNz76yNwBhuwYwRbhihLnJqM73VTCjS3UMJud9A/go-path"
)

var log = logging.Logger("core/commands/ipns")

type ResolvedPath struct {
	Path path.Path
}

const (
	recursiveOptionName      = "recursive"
	nocacheOptionName        = "nocache"
	dhtRecordCountOptionName = "dht-record-count"
	dhtTimeoutOptionName     = "dht-timeout"
)

var IpnsCmd = &cmds.Command{
	Helptext: cmdkit.HelpText{
		Tagline: "Resolve IPNS names.",
		ShortDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.
`,
		LongDescription: `
IPNS is a PKI namespace, where names are the hashes of public keys, and
the private key enables publishing new (signed) values. In both publish
and resolve, the default name used is the node's own PeerID,
which is the hash of its public key.

You can use the 'ipfs key' commands to list and generate more names and their
respective keys.

Examples:

Resolve the value of your name:

  > ipfs name resolve
  /ipfs/QmatmE9msSfkKxoffpHwNLNKgwZG8eT9Bud6YoPab52vpy

Resolve the value of another name:

  > ipfs name resolve QmaCpDMGvV2BGHeYERUEnRQAwe3N8SzbUtfsmvsqQLuvuJ
  /ipfs/QmSiTko9JZyabH56y2fussEt1A5oDqsFXB3CkvAqraFryz

Resolve the value of a dnslink:

  > ipfs name resolve ipfs.io
  /ipfs/QmaBvfZooxWkrv7D3r8LS9moNjzD2o525XMZze69hhoxf5

`,
	},

	Arguments: []cmdkit.Argument{
		cmdkit.StringArg("name", false, false, "The IPNS name to resolve. Defaults to your node's peerID."),
	},
	Options: []cmdkit.Option{
		cmdkit.BoolOption(recursiveOptionName, "r", "Resolve until the result is not an IPNS name."),
		cmdkit.BoolOption(nocacheOptionName, "n", "Do not use cached entries."),
		cmdkit.UintOption(dhtRecordCountOptionName, "dhtrc", "Number of records to request for DHT resolution."),
		cmdkit.StringOption(dhtTimeoutOptionName, "dhtt", "Max time to collect values during DHT resolution eg \"30s\". Pass 0 for no timeout."),
	},
	Run: func(req *cmds.Request, res cmds.ResponseEmitter, env cmds.Environment) {
		n, err := cmdenv.GetNode(env)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		if !n.OnlineMode() {
			err := n.SetupOfflineRouting()
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
		}

		nocache, _ := req.Options["nocache"].(bool)
		local, _ := req.Options["local"].(bool)

		// default to nodes namesys resolver
		var resolver namesys.Resolver = n.Namesys

		if local && nocache {
			res.SetError(errors.New("cannot specify both local and nocache"), cmdkit.ErrNormal)
			return
		}

		if local {
			offroute := offline.NewOfflineRouter(n.Repo.Datastore(), n.RecordValidator)
			resolver = namesys.NewIpnsResolver(offroute)
		}

		if nocache {
			resolver = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), 0)
		}

		var name string
		if len(req.Arguments) == 0 {
			if n.Identity == "" {
				res.SetError(errors.New("identity not loaded"), cmdkit.ErrNormal)
				return
			}
			name = n.Identity.Pretty()

		} else {
			name = req.Arguments[0]

			// maybe the name is key if it has no slashes
			if !strings.Contains(name, "/") {
				privKey, err := n.GetKey(name)
				if privKey != nil {
					pubKey := privKey.GetPublic()
					pid, err := peer.IDFromPublicKey(pubKey)
					if err != nil {
						res.SetError(err, cmdkit.ErrNormal)
						return
					}
					name = pid.Pretty()
				}

				if err != nil && err != keystore.ErrNoSuchKey {
					res.SetError(err, cmdkit.ErrNormal)
					return
				}
			}
		}

		recursive, _ := req.Options[recursiveOptionName].(bool)
		rc, rcok := req.Options[dhtRecordCountOptionName].(int)
		dhtt, dhttok := req.Options[dhtTimeoutOptionName].(string)

		var ropts []nsopts.ResolveOpt
		if !recursive {
			ropts = append(ropts, nsopts.Depth(1))
		}
		if rcok {
			ropts = append(ropts, nsopts.DhtRecordCount(uint(rc)))
		}
		if dhttok {
			d, err := time.ParseDuration(dhtt)
			if err != nil {
				res.SetError(err, cmdkit.ErrNormal)
				return
			}
			if d < 0 {
				res.SetError(errors.New("DHT timeout value must be >= 0"), cmdkit.ErrNormal)
				return
			}
			ropts = append(ropts, nsopts.DhtTimeout(d))
		}

		if !strings.HasPrefix(name, "/ipns/") {
			name = "/ipns/" + name
		}

		output, err := resolver.Resolve(req.Context, name, ropts...)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: better errors (in the case of not finding the name, we get "failed to find any peer in table")

		cmds.EmitOnce(res, &ResolvedPath{output})
	},
	Encoders: cmds.EncoderMap{
		cmds.Text: cmds.MakeEncoder(func(req *cmds.Request, w io.Writer, v interface{}) error {
			output, ok := v.(*ResolvedPath)
			if !ok {
				return e.TypeErr(output, v)
			}
			_, err := fmt.Fprintln(w, output.Path)
			return err
		}),
	},
	Type: ResolvedPath{},
}
