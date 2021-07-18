# go-ipfs

![banner](https://ipfs.io/ipfs/bafykbzacecaesuqmivkauix25v6i6xxxsvsrtxknhgb5zak3xxsg2nb4dhs2u/ipfs.go.png)

[![](https://img.shields.io/badge/made%20by-Protocol%20Labs-blue.svg?style=flat-square&cacheSeconds=3600)](https://protocol.ai)
[![GoDoc](https://img.shields.io/badge/godoc-reference-5272B4.svg?style=flat-square&cacheSeconds=3600)](https://godoc.org/github.com/ipfs/go-ipfs)
[![CircleCI](https://img.shields.io/circleci/build/github/ipfs/go-ipfs?style=flat-square&cacheSeconds=3600)](https://circleci.com/gh/ipfs/go-ipfs)

## What is IPFS?

IPFS is a global, versioned, peer-to-peer filesystem. It combines good ideas from previous systems such as Git, BitTorrent, Kademlia, SFS, and the Web. It is like a single BitTorrent swarm, exchanging git objects. IPFS provides an interface as simple as the HTTP web, but with permanence built-in. You can also mount the world at /ipfs.

For more info see: https://docs.ipfs.io/introduction/overview/

Before opening an issue, consider using one of the following locations to ensure you are opening your thread in the right place:
  - go-ipfs _implementation_ bugs in [this repo](https://github.com/ipfs/go-ipfs/issues).
  - Documentation issues in [ipfs/docs issues](https://github.com/ipfs/docs/issues).
  - IPFS _design_ in [ipfs/specs issues](https://github.com/ipfs/specs/issues).
  - Exploration of new ideas in [ipfs/notes issues](https://github.com/ipfs/notes/issues).
  - Ask questions and meet the rest of the community at the [IPFS Forum](https://discuss.ipfs.io).
  - Or [chat with us](https://docs.ipfs.io/community/chat/).
 
[![YouTube Channel Subscribers](https://img.shields.io/youtube/channel/subscribers/UCdjsUXJ3QawK4O5L1kqqsew?label=Subscribe%20IPFS&style=social&cacheSeconds=3600)](https://www.youtube.com/channel/UCdjsUXJ3QawK4O5L1kqqsew) [![Follow @IPFS on Twitter](https://img.shields.io/twitter/follow/IPFS?style=social&cacheSeconds=3600)](https://twitter.com/IPFS)

## Next milestones

[![GitHub milestone version 0.9.1](https://img.shields.io/github/milestones/progress/ipfs/go-ipfs/51?logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://github.com/ipfs/go-ipfs/milestone/51)
[![GitHub milestone version 0.10](https://img.shields.io/github/milestones/progress/ipfs/go-ipfs/48?logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://github.com/ipfs/go-ipfs/milestone/48)
[![GitHub milestone version 0.11](https://img.shields.io/github/milestones/progress/ipfs/go-ipfs/50?logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://github.com/ipfs/go-ipfs/milestone/50)
[![GitHub milestone version 0.12](https://img.shields.io/github/milestones/progress/ipfs/go-ipfs/49?logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://github.com/ipfs/go-ipfs/milestone/49)


## Table of Contents

- [Security Issues](#security-issues)
- [Install](#install)
  - [System Requirements](#system-requirements)
  - [Docker](#docker)
  - [Native Linux package managers](#native-linux-package-managers)
    - [ArchLinux](#archlinux)
    - [openSUSE](#opensuse)
  - [Other package managers](#other-package-managers)
    - [Nix](#nix)
    - [Guix](#guix)
    - [Solus](#solus)
    - [Snap](#snap)
  - [Windows package managers](windows-package-managers)
    - [Chocolatey](#chocolatey)
    - [Scoop](#scoop)
  - [Install prebuilt packages](#install-prebuilt-packages)  
  - [Build from Source](#build-from-source)
    - [Install Go](#install-go)
    - [Download and Compile IPFS](#download-and-compile-ipfs)
    - [Cross Compiling](#cross-compiling)
    - [OpenSSL](#openssl)
    - [Troubleshooting](#troubleshooting)
  - [Updating go-ipfs](#updating-go-ipfs)
- [Getting Started](#getting-started)
  - [Some things to try](#some-things-to-try)
  - [Usage](#usage)
  - [Troubleshooting](#troubleshooting-1)
- [Packages](#packages)
- [Development](#development)
  - [Map of go-ipfs Subsystems](#map-of-go-ipfs-subsystems)
  - [CLI, HTTP-API, Architecture Diagram](#cli-http-api-architecture-diagram)
  - [Testing](#testing)
  - [Development Dependencies](#development-dependencies)
  - [Developer Notes](#developer-notes)
- [Contributing](#contributing)
- [License](#license)

## Security Issues

The IPFS protocol and its implementations are still in heavy development. This means that there may be problems in our protocols, or there may be mistakes in our implementations. And -- though IPFS is not production-ready yet -- many people are already running nodes in their machines. So we take security vulnerabilities very seriously. If you discover a security issue, please bring it to our attention right away!

If you find a vulnerability that may affect live deployments -- for example, by exposing a remote execution exploit -- please send your report privately to security@ipfs.io. Please DO NOT file a public issue.

If the issue is a protocol weakness that cannot be immediately exploited or something not yet deployed, just discuss it openly.

## Install

The canonical download instructions for IPFS are over at: https://docs.ipfs.io/guides/guides/install/. It is **highly recommended** you follow those instructions if you are not interested in working on IPFS development.

### System Requirements

IPFS can run on most Linux, MacOS, and Windows systems. We recommend running it on a machine with at least 2 GB of RAM and 2 CPU cores (go-ipfs is highly parallel). On systems with less memory, it may not be completely stable.

If your system is resource-constrained, we recommend:

1. Installing OpenSSL and rebuilding go-ipfs manually with `make build GOTAGS=openssl`. See the [download and compile](#download-and-compile-ipfs) section for more information on compiling go-ipfs.
2. Initializing your daemon with `ipfs init --profile=lowpower`

### Docker

[![Docker Image Version (latest semver)](https://img.shields.io/docker/v/ipfs/go-ipfs?color=blue&label=go-ipfs%20docker%20image&logo=docker&sort=semver&style=for-the-badge&cacheSeconds=3600)](https://hub.docker.com/r/ipfs/go-ipfs/)

More info on how to run go-ipfs inside docker can be found [here](https://docs.ipfs.io/how-to/run-ipfs-inside-docker/).

### Native Linux package managers

- [Arch Linux](#arch-linux)
- [openSUSE](#openSUSE)

#### ArchLinux

[![go-ipfs via Community Repo](https://img.shields.io/archlinux/v/community/x86_64/go-ipfs?color=1793d1&label=go-ipfs&logo=arch-linux&style=for-the-badge&cacheSeconds=3600)](https://wiki.archlinux.org/title/IPFS)

[![go-ipfs-git via AUR](https://img.shields.io/aur/version/go-ipfs-git?color=1793d1&label=go-ipfs-git&logo=arch-linux&style=for-the-badge&cacheSeconds=3600)](https://aur.archlinux.org/packages/go-ipfs-git/)

#### openSUSE

[Community Package for go-ipfs](https://software.opensuse.org/package/go-ipfs)

### Other package managers

- [Nix](#nix)
- [Solus](#solus)
- [Guix](#guix)
- [Snap](#snap)

#### Nix

For Linux and MacOSX you can use the purely functional package manager [Nix](https://nixos.org/nix/):

```
$ nix-env -i ipfs
```

You can also install the Package by using its attribute name, which is also `ipfs`.

#### Guix

GNU's functional package manager, [Guix](https://www.gnu.org/software/guix/), also provides a go-ipfs package:

```
$ guix package -i go-ipfs
```

#### Solus

In solus, go-ipfs is available in the main repository as
[go-ipfs](https://dev.getsol.us/source/go-ipfs/repository/master/).

```
$ sudo eopkg install go-ipfs
```

You can also install it through the Solus software center.

#### Snap

With snap, in any of the [supported Linux distributions](https://snapcraft.io/docs/core/install):

```
$ sudo snap install ipfs
```

### Windows package managers

- [Chocolatey](#chocolatey)
- [Scoop](#scoop)

#### Chocolatey

[![Chocolatey Version](https://img.shields.io/chocolatey/v/go-ipfs?color=00a4ef&label=go-ipfs&logo=windows&style=for-the-badge&cacheSeconds=3600)](https://chocolatey.org/packages/go-ipfs)

#### Scoop

Scoop provides `go-ipfs` in its 'extras' bucket.
```Powershell
PS> scoop bucket add extras
PS> scoop install go-ipfs
```

### Install prebuilt packages

[![dist.ipfs.io Downloads](https://img.shields.io/github/v/release/ipfs/go-ipfs?label=dist.ipfs.io&logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://ipfs.io/ipns/dist.ipfs.io#go-ipfs)

From there:
- Click the blue "Download go-ipfs" on the right side of the page.
- Open/extract the archive.
- Move `ipfs` to your path (`install.sh` can do it for you).

You can also download go-ipfs from this project's GitHub releases page if you are unable to access dist.ipfs.io:

[![dist.ipfs.io Downloads](https://img.shields.io/github/v/release/ipfs/go-ipfs?label=GitHub%20Downloads&logo=ipfs&style=for-the-badge&cacheSeconds=3600)](https://github.com/ipfs/go-ipfs/releases)


### Build from Source

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ipfs/go-ipfs?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

go-ipfs's build system requires Go and some standard POSIX build tools:

* GNU make
* Git
* GCC (or some other go compatible C Compiler) (optional)

To build without GCC, build with `CGO_ENABLED=0` (e.g., `make build CGO_ENABLED=0`).

#### Install Go

![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/ipfs/go-ipfs?label=Requires%20Go&logo=go&style=flat-square&cacheSeconds=3600)

If you need to update: [Download latest version of Go](https://golang.org/dl/).

You'll need to add Go's bin directories to your `$PATH` environment variable e.g., by adding these lines to your `/etc/profile` (for a system-wide installation) or `$HOME/.profile`:

```
export PATH=$PATH:/usr/local/go/bin
export PATH=$PATH:$GOPATH/bin
```

(If you run into trouble, see the [Go install instructions](https://golang.org/doc/install)).

#### Download and Compile IPFS

```
$ git clone https://github.com/ipfs/go-ipfs.git

$ cd go-ipfs
$ make install
```

Alternatively, you can run `make build` to build the go-ipfs binary (storing it in `cmd/ipfs/ipfs`) without installing it.

**NOTE:** If you get an error along the lines of "fatal error: stdlib.h: No such file or directory", you're missing a C compiler. Either re-run `make` with `CGO_ENABLED=0` or install GCC.

##### Cross Compiling

Compiling for a different platform is as simple as running:

```
make build GOOS=myTargetOS GOARCH=myTargetArchitecture
```

##### OpenSSL

To build go-ipfs with OpenSSL support, append `GOTAGS=openssl` to your `make` invocation. Building with OpenSSL should significantly reduce the background CPU usage on nodes that frequently make or receive new connections.

Note: OpenSSL requires CGO support and, by default, CGO is disabled when cross-compiling. To cross-compile with OpenSSL support, you must:

1. Install a compiler toolchain for the target platform.
2. Set the `CGO_ENABLED=1` environment variable.

#### Troubleshooting

- Separate [instructions are available for building on Windows](docs/windows.md).
- `git` is required in order for `go get` to fetch all dependencies.
- Package managers often contain out-of-date `golang` packages.
  Ensure that `go version` reports at least 1.10. See above for how to install go.
- If you are interested in development, please install the development
dependencies as well.
- _WARNING_: Older versions of OSX FUSE (for Mac OS X) can cause kernel panics when mounting!-
  We strongly recommend you use the [latest version of OSX FUSE](http://osxfuse.github.io/).
  (See https://github.com/ipfs/go-ipfs/issues/177)
- Read [docs/fuse.md](docs/fuse.md) for more details on setting up FUSE (so that you can mount the filesystem).
- Shell command completions can be generated with one of the `ipfs commands completion` subcommands. Read [docs/command-completion.md](docs/command-completion.md) to learn more.
- See the [misc folder](https://github.com/ipfs/go-ipfs/tree/master/misc) for how to connect IPFS to systemd or whatever init system your distro uses.

### Updating go-ipfs

#### Using ipfs-update

IPFS has an updating tool that can be accessed through `ipfs update`. The tool is
not installed alongside IPFS in order to keep that logic independent of the main
codebase. To install `ipfs update`, [download it here](https://ipfs.io/ipns/dist.ipfs.io/#ipfs-update).

#### Downloading IPFS builds using IPFS

List the available versions of go-ipfs:

```
$ ipfs cat /ipns/dist.ipfs.io/go-ipfs/versions
```

Then, to view available builds for a version from the previous command ($VERSION):

```
$ ipfs ls /ipns/dist.ipfs.io/go-ipfs/$VERSION
```

To download a given build of a version:

```
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_darwin-386.tar.gz # darwin 32-bit build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_darwin-amd64.tar.gz # darwin 64-bit build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_freebsd-amd64.tar.gz # freebsd 64-bit build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_linux-386.tar.gz # linux 32-bit build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_linux-amd64.tar.gz # linux 64-bit build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_linux-arm.tar.gz # linux arm build
$ ipfs get /ipns/dist.ipfs.io/go-ipfs/$VERSION/go-ipfs_$VERSION_windows-amd64.zip # windows 64-bit build
```

## Getting Started

### Usage

[![docs: Command-line quick start](https://img.shields.io/static/v1?label=docs&message=Command-line%20quick%20start&color=blue&style=for-the-badge&cacheSeconds=3600)](https://docs.ipfs.io/how-to/command-line-quick-start/)
[![docs: Command-line reference](https://img.shields.io/static/v1?label=docs&message=Command-line%20reference&color=blue&style=for-the-badge&cacheSeconds=3600)](https://docs.ipfs.io/reference/cli/)

To start using IPFS, you must first initialize IPFS's config files on your
system, this is done with `ipfs init`. See `ipfs init --help` for information on
the optional arguments it takes. After initialization is complete, you can use
`ipfs mount`, `ipfs add` and any of the other commands to explore!

### Some things to try

Basic proof of 'ipfs working' locally:

    echo "hello world" > hello
    ipfs add hello
    # This should output a hash string that looks something like:
    # QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
    ipfs cat <that hash>

### Troubleshooting

If you have previously installed IPFS before and you are running into problems getting a newer version to work, try deleting (or backing up somewhere else) your IPFS config directory (~/.ipfs by default) and rerunning `ipfs init`. This will reinitialize the config file to its defaults and clear out the local datastore of any bad entries.

Please direct general questions and help requests to our [forum](https://discuss.ipfs.io) or our IRC channel (freenode #ipfs).

If you believe you've found a bug, check the [issues list](https://github.com/ipfs/go-ipfs/issues) and, if you don't see your problem there, either come talk to us on IRC (freenode #ipfs) or file an issue of your own!

## Packages

> This table is generated using the module [`package-table`](https://github.com/ipfs-shipyard/package-table) with `package-table --data=package-list.json`.

Listing of the main packages used in the IPFS ecosystem. There are also three specifications worth linking here:

| Name | CI/Travis | Coverage | Description |
| ---------|---------|---------|--------- |
| **Libp2p** |
| [`go-libp2p`](//github.com/libp2p/go-libp2p) | [![Travis CI](https://flat.badgen.net/travis/libp2p/go-libp2p/master)](https://travis-ci.com/libp2p/go-libp2p) | [![codecov](https://codecov.io/gh/libp2p/go-libp2p/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/libp2p/go-libp2p) | p2p networking library |
| [`go-libp2p-pubsub`](//github.com/libp2p/go-libp2p-pubsub) | [![Travis CI](https://flat.badgen.net/travis/libp2p/go-libp2p-pubsub/master)](https://travis-ci.com/libp2p/go-libp2p-pubsub) | [![codecov](https://codecov.io/gh/libp2p/go-libp2p-pubsub/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/libp2p/go-libp2p-pubsub) | pubsub built on libp2p |
| [`go-libp2p-kad-dht`](//github.com/libp2p/go-libp2p-kad-dht) | [![Travis CI](https://flat.badgen.net/travis/libp2p/go-libp2p-kad-dht/master)](https://travis-ci.com/libp2p/go-libp2p-kad-dht) | [![codecov](https://codecov.io/gh/libp2p/go-libp2p-kad-dht/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/libp2p/go-libp2p-kad-dht) | dht-backed router |
| [`go-libp2p-pubsub-router`](//github.com/libp2p/go-libp2p-pubsub-router) | [![Travis CI](https://flat.badgen.net/travis/libp2p/go-libp2p-pubsub-router/master)](https://travis-ci.com/libp2p/go-libp2p-pubsub-router) | [![codecov](https://codecov.io/gh/libp2p/go-libp2p-pubsub-router/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/libp2p/go-libp2p-pubsub-router) | pubsub-backed router |
| **Multiformats** |
| [`go-cid`](//github.com/ipfs/go-cid) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-cid/master)](https://travis-ci.com/ipfs/go-cid) | [![codecov](https://codecov.io/gh/ipfs/go-cid/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-cid) | CID implementation |
| [`go-multiaddr`](//github.com/multiformats/go-multiaddr) | [![Travis CI](https://flat.badgen.net/travis/multiformats/go-multiaddr/master)](https://travis-ci.com/multiformats/go-multiaddr) | [![codecov](https://codecov.io/gh/multiformats/go-multiaddr/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/multiformats/go-multiaddr) | multiaddr implementation |
| [`go-multihash`](//github.com/multiformats/go-multihash) | [![Travis CI](https://flat.badgen.net/travis/multiformats/go-multihash/master)](https://travis-ci.com/multiformats/go-multihash) | [![codecov](https://codecov.io/gh/multiformats/go-multihash/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/multiformats/go-multihash) | multihash implementation |
| [`go-multibase`](//github.com/multiformats/go-multibase) | [![Travis CI](https://flat.badgen.net/travis/multiformats/go-multibase/master)](https://travis-ci.com/multiformats/go-multibase) | [![codecov](https://codecov.io/gh/multiformats/go-multibase/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/multiformats/go-multibase) | mulitbase implementation |
| **Files** |
| [`go-unixfs`](//github.com/ipfs/go-unixfs) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-unixfs/master)](https://travis-ci.com/ipfs/go-unixfs) | [![codecov](https://codecov.io/gh/ipfs/go-unixfs/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-unixfs) | the core 'filesystem' logic |
| [`go-mfs`](//github.com/ipfs/go-mfs) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-mfs/master)](https://travis-ci.com/ipfs/go-mfs) | [![codecov](https://codecov.io/gh/ipfs/go-mfs/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-mfs) | a mutable filesystem editor for unixfs |
| [`go-ipfs-posinfo`](//github.com/ipfs/go-ipfs-posinfo) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-posinfo/master)](https://travis-ci.com/ipfs/go-ipfs-posinfo) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-posinfo/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-posinfo) | helper datatypes for the filestore |
| [`go-ipfs-chunker`](//github.com/ipfs/go-ipfs-chunker) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-chunker/master)](https://travis-ci.com/ipfs/go-ipfs-chunker) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-chunker/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-chunker) | file chunkers |
| **Exchange** |
| [`go-ipfs-exchange-interface`](//github.com/ipfs/go-ipfs-exchange-interface) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-exchange-interface/master)](https://travis-ci.com/ipfs/go-ipfs-exchange-interface) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-exchange-interface/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-exchange-interface) | exchange service interface |
| [`go-ipfs-exchange-offline`](//github.com/ipfs/go-ipfs-exchange-offline) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-exchange-offline/master)](https://travis-ci.com/ipfs/go-ipfs-exchange-offline) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-exchange-offline/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-exchange-offline) | (dummy) offline implementation of the exchange service |
| [`go-bitswap`](//github.com/ipfs/go-bitswap) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-bitswap/master)](https://travis-ci.com/ipfs/go-bitswap) | [![codecov](https://codecov.io/gh/ipfs/go-bitswap/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-bitswap) | bitswap protocol implementation |
| [`go-blockservice`](//github.com/ipfs/go-blockservice) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-blockservice/master)](https://travis-ci.com/ipfs/go-blockservice) | [![codecov](https://codecov.io/gh/ipfs/go-blockservice/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-blockservice) | service that plugs a blockstore and an exchange together |
| **Datastores** |
| [`go-datastore`](//github.com/ipfs/go-datastore) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-datastore/master)](https://travis-ci.com/ipfs/go-datastore) | [![codecov](https://codecov.io/gh/ipfs/go-datastore/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-datastore) | datastore interfaces, adapters, and basic implementations |
| [`go-ipfs-ds-help`](//github.com/ipfs/go-ipfs-ds-help) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-ds-help/master)](https://travis-ci.com/ipfs/go-ipfs-ds-help) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-ds-help/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-ds-help) | datastore utility functions |
| [`go-ds-flatfs`](//github.com/ipfs/go-ds-flatfs) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ds-flatfs/master)](https://travis-ci.com/ipfs/go-ds-flatfs) | [![codecov](https://codecov.io/gh/ipfs/go-ds-flatfs/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ds-flatfs) | a filesystem-based datastore |
| [`go-ds-measure`](//github.com/ipfs/go-ds-measure) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ds-measure/master)](https://travis-ci.com/ipfs/go-ds-measure) | [![codecov](https://codecov.io/gh/ipfs/go-ds-measure/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ds-measure) | a metric-collecting database adapter |
| [`go-ds-leveldb`](//github.com/ipfs/go-ds-leveldb) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ds-leveldb/master)](https://travis-ci.com/ipfs/go-ds-leveldb) | [![codecov](https://codecov.io/gh/ipfs/go-ds-leveldb/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ds-leveldb) | a leveldb based datastore |
| [`go-ds-badger`](//github.com/ipfs/go-ds-badger) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ds-badger/master)](https://travis-ci.com/ipfs/go-ds-badger) | [![codecov](https://codecov.io/gh/ipfs/go-ds-badger/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ds-badger) | a badgerdb based datastore |
| **Namesys** |
| [`go-ipns`](//github.com/ipfs/go-ipns) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipns/master)](https://travis-ci.com/ipfs/go-ipns) | [![codecov](https://codecov.io/gh/ipfs/go-ipns/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipns) | IPNS datastructures and validation logic |
| **Repo** |
| [`go-ipfs-config`](//github.com/ipfs/go-ipfs-config) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-config/master)](https://travis-ci.com/ipfs/go-ipfs-config) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-config/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-config) | go-ipfs config file definitions |
| [`go-fs-lock`](//github.com/ipfs/go-fs-lock) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-fs-lock/master)](https://travis-ci.com/ipfs/go-fs-lock) | [![codecov](https://codecov.io/gh/ipfs/go-fs-lock/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-fs-lock) | lockfile management functions |
| [`fs-repo-migrations`](//github.com/ipfs/fs-repo-migrations) | [![Travis CI](https://flat.badgen.net/travis/ipfs/fs-repo-migrations/master)](https://travis-ci.com/ipfs/fs-repo-migrations) | [![codecov](https://codecov.io/gh/ipfs/fs-repo-migrations/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/fs-repo-migrations) | repo migrations |
| **IPLD** |
| [`go-block-format`](//github.com/ipfs/go-block-format) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-block-format/master)](https://travis-ci.com/ipfs/go-block-format) | [![codecov](https://codecov.io/gh/ipfs/go-block-format/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-block-format) | block interfaces and implementations |
| [`go-ipfs-blockstore`](//github.com/ipfs/go-ipfs-blockstore) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-blockstore/master)](https://travis-ci.com/ipfs/go-ipfs-blockstore) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-blockstore/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-blockstore) | blockstore interfaces and implementations |
| [`go-ipld-format`](//github.com/ipfs/go-ipld-format) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipld-format/master)](https://travis-ci.com/ipfs/go-ipld-format) | [![codecov](https://codecov.io/gh/ipfs/go-ipld-format/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipld-format) | IPLD interfaces |
| [`go-ipld-cbor`](//github.com/ipfs/go-ipld-cbor) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipld-cbor/master)](https://travis-ci.com/ipfs/go-ipld-cbor) | [![codecov](https://codecov.io/gh/ipfs/go-ipld-cbor/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipld-cbor) | IPLD-CBOR implementation |
| [`go-ipld-git`](//github.com/ipfs/go-ipld-git) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipld-git/master)](https://travis-ci.com/ipfs/go-ipld-git) | [![codecov](https://codecov.io/gh/ipfs/go-ipld-git/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipld-git) | IPLD-Git implementation |
| [`go-merkledag`](//github.com/ipfs/go-merkledag) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-merkledag/master)](https://travis-ci.com/ipfs/go-merkledag) | [![codecov](https://codecov.io/gh/ipfs/go-merkledag/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-merkledag) | IPLD-Merkledag implementation (and then some) |
| **Commands** |
| [`go-ipfs-cmds`](//github.com/ipfs/go-ipfs-cmds) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-cmds/master)](https://travis-ci.com/ipfs/go-ipfs-cmds) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-cmds/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-cmds) | CLI & HTTP commands library |
| [`go-ipfs-files`](//github.com/ipfs/go-ipfs-files) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-files/master)](https://travis-ci.com/ipfs/go-ipfs-files) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-files/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-files) | CLI & HTTP commands library |
| [`go-ipfs-api`](//github.com/ipfs/go-ipfs-api) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-api/master)](https://travis-ci.com/ipfs/go-ipfs-api) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-api/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-api) | an old, stable shell for the IPFS HTTP API |
| [`go-ipfs-http-client`](//github.com/ipfs/go-ipfs-http-client) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-http-client/master)](https://travis-ci.com/ipfs/go-ipfs-http-client) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-http-client/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-http-client) | a new, unstable shell for the IPFS HTTP API |
| [`interface-go-ipfs-core`](//github.com/ipfs/interface-go-ipfs-core) | [![Travis CI](https://flat.badgen.net/travis/ipfs/interface-go-ipfs-core/master)](https://travis-ci.com/ipfs/interface-go-ipfs-core) | [![codecov](https://codecov.io/gh/ipfs/interface-go-ipfs-core/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/interface-go-ipfs-core) | core go-ipfs API interface definitions |
| **Metrics & Logging** |
| [`go-metrics-interface`](//github.com/ipfs/go-metrics-interface) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-metrics-interface/master)](https://travis-ci.com/ipfs/go-metrics-interface) | [![codecov](https://codecov.io/gh/ipfs/go-metrics-interface/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-metrics-interface) | metrics collection interfaces |
| [`go-metrics-prometheus`](//github.com/ipfs/go-metrics-prometheus) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-metrics-prometheus/master)](https://travis-ci.com/ipfs/go-metrics-prometheus) | [![codecov](https://codecov.io/gh/ipfs/go-metrics-prometheus/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-metrics-prometheus) | prometheus-backed metrics collector |
| [`go-log`](//github.com/ipfs/go-log) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-log/master)](https://travis-ci.com/ipfs/go-log) | [![codecov](https://codecov.io/gh/ipfs/go-log/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-log) | logging framework |
| **Generics/Utils** |
| [`go-ipfs-routing`](//github.com/ipfs/go-ipfs-routing) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-routing/master)](https://travis-ci.com/ipfs/go-ipfs-routing) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-routing/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-routing) | routing (content, peer, value) helpers |
| [`go-ipfs-util`](//github.com/ipfs/go-ipfs-util) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-util/master)](https://travis-ci.com/ipfs/go-ipfs-util) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-util/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-util) | the kitchen sink |
| [`go-ipfs-addr`](//github.com/ipfs/go-ipfs-addr) | [![Travis CI](https://flat.badgen.net/travis/ipfs/go-ipfs-addr/master)](https://travis-ci.com/ipfs/go-ipfs-addr) | [![codecov](https://codecov.io/gh/ipfs/go-ipfs-addr/branch/master/graph/badge.svg?style=flat-square)](https://codecov.io/gh/ipfs/go-ipfs-addr) | utility functions for parsing IPFS multiaddrs |

For brevity, we've omitted most go-libp2p, go-ipld, and go-multiformats packages. These package tables can be found in their respective project's READMEs:

* [go-libp2p](https://github.com/libp2p/go-libp2p#packages)
* [go-ipld](https://github.com/ipld/go-ipld#packages)

## Development

Some places to get you started on the codebase:

- Main file: [./cmd/ipfs/main.go](https://github.com/ipfs/go-ipfs/blob/master/cmd/ipfs/main.go)
- CLI Commands: [./core/commands/](https://github.com/ipfs/go-ipfs/tree/master/core/commands)
- Bitswap (the data trading engine): [go-bitswap](https://github.com/ipfs/go-bitswap)
- libp2p
  - libp2p: https://github.com/libp2p/go-libp2p
  - DHT: https://github.com/libp2p/go-libp2p-kad-dht
  - PubSub: https://github.com/libp2p/go-libp2p-pubsub
- [IPFS : The `Add` command demystified](https://github.com/ipfs/go-ipfs/tree/master/docs/add-code-flow.md)

### Map of go-ipfs Subsystems
**WIP**: This is a high-level architecture diagram of the various sub-systems of go-ipfs. To be updated with how they interact. Anyone who has suggestions is welcome to comment [here](https://docs.google.com/drawings/d/1OVpBT2q-NtSJqlPX3buvjYhOnWfdzb85YEsM_njesME/edit) on how we can improve this!
<img src="https://docs.google.com/drawings/d/e/2PACX-1vS_n1FvSu6mdmSirkBrIIEib2gqhgtatD9awaP2_WdrGN4zTNeg620XQd9P95WT-IvognSxIIdCM5uE/pub?w=1446&amp;h=1036">

### CLI, HTTP-API, Architecture Diagram

![](./docs/cli-http-api-core-diagram.png)

> [Origin](https://github.com/ipfs/pm/pull/678#discussion_r210410924)

Description: Dotted means "likely going away". The "Legacy" parts are thin wrappers around some commands to translate between the new system and the old system. The grayed-out parts on the "daemon" diagram are there to show that the code is all the same, it's just that we turn some pieces on and some pieces off depending on whether we're running on the client or the server.

### Testing

```
make test
```

### Development Dependencies

If you make changes to the protocol buffers, you will need to install the [protoc compiler](https://github.com/google/protobuf).

### Developer Notes

Find more documentation for developers on [docs](./docs)

## Contributing

[![](https://cdn.rawgit.com/jbenet/contribute-ipfs-gif/master/img/contribute.gif)](https://github.com/ipfs/community/blob/master/CONTRIBUTING.md)

We ❤️ all [our contributors](docs/AUTHORS); this project wouldn’t be what it is without you! If you want to help out, please see [CONTRIBUTING.md](CONTRIBUTING.md).

This repository falls under the IPFS [Code of Conduct](https://github.com/ipfs/community/blob/master/code-of-conduct.md).

Please reach out to us in one [chat](https://docs.ipfs.io/community/chat/) rooms.

## License

The go-ipfs project is dual-licensed under Apache 2.0 and MIT terms:

- Apache License, Version 2.0, ([LICENSE-APACHE](https://github.com/ipfs/go-ipfs/blob/master/LICENSE-APACHE) or http://www.apache.org/licenses/LICENSE-2.0)
- MIT license ([LICENSE-MIT](https://github.com/ipfs/go-ipfs/blob/master/LICENSE-MIT) or http://opensource.org/licenses/MIT)
