module github.com/ipfs/go-ipfs

require (
	bazil.org/fuse v0.0.0-20180421153158-65cc252bf669
	github.com/Kubuxu/go-os-helper v0.0.1
	github.com/Kubuxu/gocovmerge v0.0.0-20161216165753-7ecaa51963cd
	github.com/blang/semver v3.5.1+incompatible
	github.com/bren2010/proquint v0.0.0-20160323162903-38337c27106d
	github.com/cenkalti/backoff v2.1.1+incompatible
	github.com/dustin/go-humanize v1.0.0
	github.com/elgris/jsondiff v0.0.0-20160530203242-765b5c24c302
	github.com/fatih/color v1.7.0 // indirect
	github.com/fsnotify/fsnotify v1.4.7
	github.com/go-bindata/go-bindata v3.1.1+incompatible
	github.com/go-critic/go-critic v0.0.0-20181204210945-ee9bf5809ead // indirect
	github.com/gogo/protobuf v1.2.1
	github.com/golangci/golangci-lint v1.16.1-0.20190425135923-692dacb773b7
	github.com/hashicorp/go-multierror v1.0.0
	github.com/hashicorp/golang-lru v0.5.1
	github.com/ipfs/go-bitswap v0.1.5
	github.com/ipfs/go-block-format v0.0.2
	github.com/ipfs/go-blockservice v0.1.0
	github.com/ipfs/go-cid v0.0.2
	github.com/ipfs/go-cidutil v0.0.2
	github.com/ipfs/go-datastore v0.0.5
	github.com/ipfs/go-detect-race v0.0.1
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ds-flatfs v0.0.2
	github.com/ipfs/go-ds-leveldb v0.0.2
	github.com/ipfs/go-ds-measure v0.0.1
	github.com/ipfs/go-fs-lock v0.0.1
	github.com/ipfs/go-ipfs-blockstore v0.0.1
	github.com/ipfs/go-ipfs-blocksutil v0.0.1
	github.com/ipfs/go-ipfs-chunker v0.0.1
	github.com/ipfs/go-ipfs-cmds v0.0.10
	github.com/ipfs/go-ipfs-config v0.0.6
	github.com/ipfs/go-ipfs-ds-help v0.0.1
	github.com/ipfs/go-ipfs-exchange-interface v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-files v0.0.3
	github.com/ipfs/go-ipfs-posinfo v0.0.1
	github.com/ipfs/go-ipfs-routing v0.1.0
	github.com/ipfs/go-ipfs-util v0.0.1
	github.com/ipfs/go-ipld-cbor v0.0.2
	github.com/ipfs/go-ipld-format v0.0.2
	github.com/ipfs/go-ipld-git v0.0.2
	github.com/ipfs/go-ipns v0.0.1
	github.com/ipfs/go-log v0.0.1
	github.com/ipfs/go-merkledag v0.1.0
	github.com/ipfs/go-metrics-interface v0.0.1
	github.com/ipfs/go-metrics-prometheus v0.0.2
	github.com/ipfs/go-mfs v0.1.0
	github.com/ipfs/go-path v0.0.7
	github.com/ipfs/go-unixfs v0.0.8
	github.com/ipfs/go-verifcid v0.0.1
	github.com/ipfs/hang-fds v0.0.1
	github.com/ipfs/interface-go-ipfs-core v0.1.0
	github.com/ipfs/iptb v1.4.0
	github.com/ipfs/iptb-plugins v0.1.0
	github.com/jbenet/go-is-domain v1.0.2
	github.com/jbenet/go-random v0.0.0-20190219211222-123a90aedc0c
	github.com/jbenet/go-random-files v0.0.0-20190219210431-31b3f20ebded
	github.com/jbenet/go-temp-err-catcher v0.0.0-20150120210811-aac704a3f4f2
	github.com/jbenet/goprocess v0.1.3
	github.com/libp2p/go-eventbus v0.0.3 // indirect
	github.com/libp2p/go-libp2p v0.2.0
	github.com/libp2p/go-libp2p-autonat-svc v0.1.0
	github.com/libp2p/go-libp2p-circuit v0.1.0
	github.com/libp2p/go-libp2p-connmgr v0.1.0
	github.com/libp2p/go-libp2p-core v0.0.6
	github.com/libp2p/go-libp2p-discovery v0.1.0
	github.com/libp2p/go-libp2p-http v0.1.2
	github.com/libp2p/go-libp2p-kad-dht v0.1.1
	github.com/libp2p/go-libp2p-kbucket v0.2.0
	github.com/libp2p/go-libp2p-loggables v0.1.0
	github.com/libp2p/go-libp2p-mplex v0.2.1
	github.com/libp2p/go-libp2p-peerstore v0.1.2-0.20190621130618-cfa9bb890c1a
	github.com/libp2p/go-libp2p-pnet v0.1.0
	github.com/libp2p/go-libp2p-pubsub v0.1.0
	github.com/libp2p/go-libp2p-pubsub-router v0.1.0
	github.com/libp2p/go-libp2p-quic-transport v0.1.1
	github.com/libp2p/go-libp2p-record v0.1.0
	github.com/libp2p/go-libp2p-routing-helpers v0.1.0
	github.com/libp2p/go-libp2p-secio v0.1.0
	github.com/libp2p/go-libp2p-swarm v0.1.1
	github.com/libp2p/go-libp2p-testing v0.0.4
	github.com/libp2p/go-libp2p-tls v0.1.0
	github.com/libp2p/go-libp2p-yamux v0.2.1
	github.com/libp2p/go-maddr-filter v0.0.5
	github.com/mattn/go-runewidth v0.0.4 // indirect
	github.com/mgutz/ansi v0.0.0-20170206155736-9520e82c474b // indirect
	github.com/mitchellh/go-homedir v1.1.0
	github.com/mr-tron/base58 v1.1.2
	github.com/multiformats/go-multiaddr v0.0.4
	github.com/multiformats/go-multiaddr-dns v0.0.2
	github.com/multiformats/go-multiaddr-net v0.0.1
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.5
	github.com/opentracing/opentracing-go v1.1.0
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3
	github.com/prometheus/procfs v0.0.0-20190519111021-9935e8e0588d // indirect
	github.com/syndtr/goleveldb v1.0.0
	github.com/whyrusleeping/base32 v0.0.0-20170828182744-c30ac30633cc
	github.com/whyrusleeping/go-sysinfo v0.0.0-20190219211824-4a357d4b90b1
	github.com/whyrusleeping/multiaddr-filter v0.0.0-20160516205228-e903e4adabd7
	github.com/whyrusleeping/tar-utils v0.0.0-20180509141711-8c6c8ba81d5c
	go.uber.org/atomic v1.4.0 // indirect
	go.uber.org/dig v1.7.0 // indirect
	go.uber.org/fx v1.9.0
	go.uber.org/goleak v0.10.0 // indirect
	go.uber.org/multierr v1.1.0 // indirect
	go4.org v0.0.0-20190313082347-94abd6928b1d // indirect
	golang.org/x/sync v0.0.0-20190423024810-112230192c58 // indirect
	golang.org/x/sys v0.0.0-20190626221950-04f50cda93cb
	google.golang.org/appengine v1.4.0 // indirect
	gopkg.in/cheggaaa/pb.v1 v1.0.28
	gotest.tools/gotestsum v0.3.4
)

go 1.12
