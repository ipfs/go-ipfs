package commands

import (
	"errors"
	"io"
	"strings"

	cmds "github.com/ipfs/go-ipfs/commands"
	e "github.com/ipfs/go-ipfs/core/commands/e"
	keystore "github.com/ipfs/go-ipfs/keystore"
	namesys "github.com/ipfs/go-ipfs/namesys"

	offline "gx/ipfs/QmZRcGYvxdauCd7hHnMYLYqcZRaDjv24c7eUNyJojAcdBb/go-ipfs-routing/offline"
	peer "gx/ipfs/QmZoWKhxUmZ2seW4BzX6fJkNR8hh9PsGModr7q171yq2SS/go-libp2p-peer"
	"gx/ipfs/QmceUdzxkimdYsgtX733uNgzf1DLHyBKN6ehGSp85ayppM/go-ipfs-cmdkit"
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
		cmdkit.BoolOption("recursive", "r", "Resolve until the result is not an IPNS name."),
		cmdkit.BoolOption("nocache", "n", "Do not use cached entries."),
	},
	Run: func(req cmds.Request, res cmds.Response) {

		n, err := req.InvocContext().GetNode()
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

		nocache, _, _ := req.Option("nocache").Bool()
		local, _, _ := req.Option("local").Bool()

		// default to nodes namesys resolver
		var resolver namesys.Resolver = n.Namesys

		if local && nocache {
			res.SetError(errors.New("cannot specify both local and nocache"), cmdkit.ErrNormal)
			return
		}

		if local {
			offroute := offline.NewOfflineRouter(n.Repo.Datastore(), n.PrivateKey)
			resolver = namesys.NewRoutingResolver(offroute, 0)
		}

		if nocache {
			resolver = namesys.NewNameSystem(n.Routing, n.Repo.Datastore(), 0)
		}

		var name string
		if len(req.Arguments()) == 0 {
			if n.Identity == "" {
				res.SetError(errors.New("identity not loaded"), cmdkit.ErrNormal)
				return
			}
			name = n.Identity.Pretty()

		} else {
			name = req.Arguments()[0]
		}

		// if name has no slashes it might be a key
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

		recursive, _, _ := req.Option("recursive").Bool()
		depth := 1
		if recursive {
			depth = namesys.DefaultDepthLimit
		}

		if !strings.HasPrefix(name, "/ipns/") {
			name = "/ipns/" + name
		}

		output, err := resolver.ResolveN(req.Context(), name, depth)
		if err != nil {
			res.SetError(err, cmdkit.ErrNormal)
			return
		}

		// TODO: better errors (in the case of not finding the name, we get "failed to find any peer in table")

		res.SetOutput(&ResolvedPath{output})
	},
	Marshalers: cmds.MarshalerMap{
		cmds.Text: func(res cmds.Response) (io.Reader, error) {
			v, err := unwrapOutput(res.Output())
			if err != nil {
				return nil, err
			}

			output, ok := v.(*ResolvedPath)
			if !ok {
				return nil, e.TypeErr(output, v)
			}
			return strings.NewReader(output.Path.String() + "\n"), nil
		},
	},
	Type: ResolvedPath{},
}
