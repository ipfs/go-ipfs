# ipfs implementation in go. [![GoDoc](https://godoc.org/github.com/jbenet/go-ipfs?status.svg)](https://godoc.org/github.com/jbenet/go-ipfs) [![Build Status](https://travis-ci.org/jbenet/go-ipfs.svg?branch=master)](https://travis-ci.org/jbenet/go-ipfs)

See: https://github.com/jbenet/ipfs

Please put all issues regarding IPFS _design_ in the
[ipfs repo issues](https://github.com/jbenet/ipfs/issues).
Please put all issues regarding go IPFS _implementation_ in [this repo](https://github.com/jbenet/go-ipfs/issues).

## Install

[Install Go 1.2+](http://golang.org/doc/install). Then:

```
go get github.com/jbenet/go-ipfs/cmd/ipfs
cd $GOPATH/src/github.com/jbenet/go-ipfs/cmd/ipfs
go install
```

NOTES:

* `git` and mercurial (`hg`) are required in order for `go get` to fetch
all dependencies.
* Package managers often contain out-of-date `golang` packages.
  Compilation from source is recommended.
* go-ipfs depends on cgo. In case you've disabled cgo, you'll need to
  compile with `CGO_ENABLED=1`
* If you are interested in development, please install the development
dependencies as well.
* **WARNING: older versions of OSX FUSE can cause kernel panics on Mac when mounting!**
  * To install or upgrade with a binary package, download the latest version of [OSX FUSE](OSX FUSE).
  * Alternatively, you can install FUSE with brew: `brew update && brew install osxfuse`. If you receive the error `osxfuse: osxfuse is already installed from the binary distribution and conflicts with this formula.`, then Uninstall OSX FUSE binary from System Preferences and attempt to `brew install osxfuse` again.


## Usage

```
ipfs - global versioned p2p merkledag file system

Basic commands:

    add <path>    Add an object to ipfs.
    cat <ref>     Show ipfs object data.
    ls <ref>      List links from an object.
    refs <ref>    List link hashes from an object.

Tool commands:

    config        Manage configuration.
    version       Show ipfs version information.
    commands      List all available commands.

Advanced Commands:

    mount         Mount an ipfs read-only mountpoint.
    serve         Serve an interface to ipfs.

Use "ipfs help <command>" for more information about a command.
```

## Getting Started
To start using ipfs, you must first initialize ipfs's config files on your
system, this is done with `ipfs init`. See `ipfs help init` for information on
arguments it takes. After initialization is complete, you can use `ipfs mount`,
`ipfs add` and any of the other commands to explore!


NOTE: if you have previously installed ipfs before and you are running into
problems getting it to work, try deleting (or backing up somewhere else) your
config directory (~/.go-ipfs by default) and rerunning `ipfs init`.


## Contributing

go-ipfs is MIT licensed open source software. We welcome contributions big and
small! Please make sure to check the
[issues](https://github.com/jbenet/go-ipfs/issues). Search the closed ones
before reporting things, and help us with the open ones.

Guidelines:

- see the [dev pseudo-roadmap](dev.md)
- please adhere to the protocol described in [the main ipfs repo](https://github.com/jbenet/ipfs) and [paper](http://static.benet.ai/t/ipfs.pdf).
- please make branches + pull-request, even if working on the main repository
- ask questions or talk about things in [Issues](https://github.com/jbenet/go-ipfs/issues) or #ipfs on freenode.
- ensure you are able to contribute (no legal issues please-- we'll probably setup a CLA)
- run `go fmt` before pushing any code
- run `golint` and `go vet` too -- some things (like protobuf files) are expected to fail.
- if you'd like to work on ipfs part-time (20+ hrs/wk) or full-time (40+ hrs/wk), contact [@jbenet](https://github.com/jbenet)
- have fun!

## Todo

IPFS is nearing an alpha release. Things left to be done are all marked as [Issues](https://github.com/jbenet/go-ipfs/issues)

## Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://code.google.com/p/protobuf/downloads/list).

## License

MIT
