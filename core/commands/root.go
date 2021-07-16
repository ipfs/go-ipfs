package commands

import (
	"errors"

	cmdenv "github.com/ipfs/go-ipfs/core/commands/cmdenv"
	dag "github.com/ipfs/go-ipfs/core/commands/dag"
	name "github.com/ipfs/go-ipfs/core/commands/name"
	ocmd "github.com/ipfs/go-ipfs/core/commands/object"
	"github.com/ipfs/go-ipfs/core/commands/pin"
	unixfs "github.com/ipfs/go-ipfs/core/commands/unixfs"

	cmds "github.com/ipfs/go-ipfs-cmds"
	logging "github.com/ipfs/go-log"
)

var log = logging.Logger("core/commands")

var ErrNotOnline = errors.New("this command must be run in online mode. Try running 'ipfs daemon' first")

const (
	ConfigOption  = "config"
	DebugOption   = "debug"
	LocalOption   = "local" // DEPRECATED: use OfflineOption
	OfflineOption = "offline"
	ApiOption     = "api"
)

var Root = &cmds.Command{
	Helptext: cmds.HelpText{
		Tagline:  "Global p2p merkle-dag filesystem.",
		Synopsis: "ipfs [--config=<config> | -c] [--debug | -D] [--help] [-h] [--api=<api>] [--offline] [--cid-base=<base>] [--upgrade-cidv0-in-output] [--encoding=<encoding> | --enc] [--timeout=<timeout>] <command> ...",
		Subcommands: `
BASIC COMMANDS
  init          Initialize local IPFS configuration
  add <path>    Add a file to IPFS
  cat <ref>     Show IPFS object data
  get <ref>     Download IPFS objects
  ls <ref>      List links from an object
  refs <ref>    List hashes of links from an object

DATA STRUCTURE COMMANDS
  dag           Interact with IPLD DAG nodes
  files         Interact with files as if they were a unix filesystem
  block         Interact with raw blocks in the datastore

TEXT ENCODING COMMANDS
  cid           Convert and discover properties of CIDs
  multibase     Encode and decode data with Multibase format

ADVANCED COMMANDS
  daemon        Start a long-running daemon process
  mount         Mount an IPFS read-only mount point
  resolve       Resolve any type of name
  name          Publish and resolve IPNS names
  key           Create and list IPNS name keypairs
  dns           Resolve DNS links
  pin           Pin objects to local storage
  repo          Manipulate the IPFS repository
  stats         Various operational stats
  p2p           Libp2p stream mounting
  filestore     Manage the filestore (experimental)

NETWORK COMMANDS
  id            Show info about IPFS peers
  bootstrap     Add or remove bootstrap peers
  swarm         Manage connections to the p2p network
  dht           Query the DHT for values or peers
  ping          Measure the latency of a connection
  diag          Print diagnostics
  bitswap       Inspect bitswap state
  pubsub        Send and receive messages via pubsub

TOOL COMMANDS
  config        Manage configuration
  version       Show IPFS version information
  update        Download and apply go-ipfs updates
  commands      List all available commands
  log           Manage and show logs of running daemon

Use 'ipfs <command> --help' to learn more about each command.

ipfs uses a repository in the local file system. By default, the repo is
located at ~/.ipfs. To change the repo location, set the $IPFS_PATH
environment variable:

  export IPFS_PATH=/path/to/ipfsrepo

EXIT STATUS

The CLI will exit with one of the following values:

0     Successful execution.
1     Failed executions.
`,
	},
	Options: []cmds.Option{
		cmds.StringOption(ConfigOption, "c", "Path to the configuration file to use."),
		cmds.BoolOption(DebugOption, "D", "Operate in debug mode."),
		cmds.BoolOption(cmds.OptLongHelp, "Show the full command help text."),
		cmds.BoolOption(cmds.OptShortHelp, "Show a short version of the command help text."),
		cmds.BoolOption(LocalOption, "L", "Run the command locally, instead of using the daemon. DEPRECATED: use --offline."),
		cmds.BoolOption(OfflineOption, "Run the command offline."),
		cmds.StringOption(ApiOption, "Use a specific API instance (defaults to /ip4/127.0.0.1/tcp/5001)"),

		// global options, added to every command
		cmdenv.OptionCidBase,
		cmdenv.OptionUpgradeCidV0InOutput,

		cmds.OptionEncodingType,
		cmds.OptionStreamChannels,
		cmds.OptionTimeout,
	},
}

// commandsDaemonCmd is the "ipfs commands" command for daemon
var CommandsDaemonCmd = CommandsCmd(Root)

var rootSubcommands = map[string]*cmds.Command{
	"add":       AddCmd,
	"bitswap":   BitswapCmd,
	"block":     BlockCmd,
	"cat":       CatCmd,
	"commands":  CommandsDaemonCmd,
	"files":     FilesCmd,
	"filestore": FileStoreCmd,
	"get":       GetCmd,
	"pubsub":    PubsubCmd,
	"repo":      RepoCmd,
	"stats":     StatsCmd,
	"bootstrap": BootstrapCmd,
	"config":    ConfigCmd,
	"dag":       dag.DagCmd,
	"dht":       DhtCmd,
	"diag":      DiagCmd,
	"dns":       DNSCmd,
	"id":        IDCmd,
	"key":       KeyCmd,
	"log":       LogCmd,
	"ls":        LsCmd,
	"mount":     MountCmd,
	"name":      name.NameCmd,
	"object":    ocmd.ObjectCmd,
	"pin":       pin.PinCmd,
	"ping":      PingCmd,
	"p2p":       P2PCmd,
	"refs":      RefsCmd,
	"resolve":   ResolveCmd,
	"swarm":     SwarmCmd,
	"tar":       TarCmd,
	"file":      unixfs.UnixFSCmd,
	"update":    ExternalBinary("Please see https://git.io/fjylH for installation instructions."),
	"urlstore":  urlStoreCmd,
	"version":   VersionCmd,
	"shutdown":  daemonShutdownCmd,
	"cid":       CidCmd,
	"multibase": MbaseCmd,
}

// RootRO is the readonly version of Root
var RootRO = &cmds.Command{}

var CommandsDaemonROCmd = CommandsCmd(RootRO)

// RefsROCmd is `ipfs refs` command
var RefsROCmd = &cmds.Command{}

// VersionROCmd is `ipfs version` command (without deps).
var VersionROCmd = &cmds.Command{}

var rootROSubcommands = map[string]*cmds.Command{
	"commands": CommandsDaemonROCmd,
	"cat":      CatCmd,
	"block": {
		Subcommands: map[string]*cmds.Command{
			"stat": blockStatCmd,
			"get":  blockGetCmd,
		},
	},
	"get": GetCmd,
	"dns": DNSCmd,
	"ls":  LsCmd,
	"name": {
		Subcommands: map[string]*cmds.Command{
			"resolve": name.IpnsCmd,
		},
	},
	"object": {
		Subcommands: map[string]*cmds.Command{
			"data":  ocmd.ObjectDataCmd,
			"links": ocmd.ObjectLinksCmd,
			"get":   ocmd.ObjectGetCmd,
			"stat":  ocmd.ObjectStatCmd,
		},
	},
	"dag": {
		Subcommands: map[string]*cmds.Command{
			"get":     dag.DagGetCmd,
			"resolve": dag.DagResolveCmd,
			"stat":    dag.DagStatCmd,
			"export":  dag.DagExportCmd,
		},
	},
	"resolve": ResolveCmd,
}

func init() {
	Root.ProcessHelp()
	*RootRO = *Root

	// this was in the big map definition above before,
	// but if we leave it there lgc.NewCommand will be executed
	// before the value is updated (:/sanitize readonly refs command/)

	// sanitize readonly refs command
	*RefsROCmd = *RefsCmd
	RefsROCmd.Subcommands = map[string]*cmds.Command{}
	rootROSubcommands["refs"] = RefsROCmd

	// sanitize readonly version command (no need to expose precise deps)
	*VersionROCmd = *VersionCmd
	VersionROCmd.Subcommands = map[string]*cmds.Command{}
	rootROSubcommands["version"] = VersionROCmd

	Root.Subcommands = rootSubcommands
	RootRO.Subcommands = rootROSubcommands
}

type MessageOutput struct {
	Message string
}
