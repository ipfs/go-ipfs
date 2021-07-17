# Manual dependency injection and spawning multiple nodes


By the end of this tutorial, you will learn how to:

- Manually inject dependencies with your desired configuration to a node.
- Spawn two different IPFS nodes with the same configuration in different ports.
- Generate a random file and add it from one node.
- Retrieve the added file from the other node.

All of this using only golang!

In order to complete this tutorial, you will need:
- golang installed on your machine. See how at https://golang.org/doc/install
- git installed on your machine (so that go can download the repo and the necessary dependencies). See how at https://git-scm.com/downloads
- IPFS Desktop (for convenience) installed and running on your machine. See how at https://github.com/ipfs-shipyard/ipfs-desktop#ipfs-desktop

**Disclaimer**: This example was an attempt to understand how to manually inject dependencies to spawn an IPFS node with custom configurations
without having to rely on `repo.Repo` and `BuildCfg`.
For this specific example the focus was on being able to initialize our own `exchange.Interface` and set IPFS nodes on different ports. Expect in 
the future further improvements to this example in order to clean the code and include additional custom configurations. The example is inspired by the [following function](../../../core/builder.go#L27)

## Getting started

**Note:** Make sure you have [![](https://img.shields.io/badge/go-%3E%3D1.13.0-blue.svg?style=flat-square)](https://golang.org/dl/) installed.

Download go-ipfs and jump into the example folder:

```
> go get -u github.com/ipfs/go-ipfs
cd $GOPATH/src/github.com/ipfs/go-ipfs/docs/examples/dep-injection-multiple-nodes
```

## Running the example as-is

To run the example, simply do:

```
> go run main.go
```

You should see the following as output:

```
[*] Spawned first node listening at:  [/ip4/192.168.0.56/tcp/36911 /ip4/127.0.0.1/tcp/36911 /ip6/::1/tcp/39831 /ip4/192.168.0.56/udp/60407/quic /ip4/127.0.0.1/udp/60407/quic /ip6/::1/udp/41561/quic /ip4/2.137.154.240/udp/60407/quic /ip4/2.137.154.240/tcp/36911]
[*] Spawned first node listening at:  [/ip6/::1/udp/41482/quic /ip4/192.168.0.56/tcp/42321 /ip4/127.0.0.1/tcp/42321 /ip6/::1/tcp/34749 /ip4/192.168.0.56/udp/37708/quic /ip4/127.0.0.1/udp/37708/quic]
[*] Connected fron node1 to node2
[*] Added a test file to the network: /ipfs/QmXG1dKK7B4srPsiiCi6xZ4HkvpShKoDxuBr7BoNEGic2M
[*] Searching for /ipfs/QmXG1dKK7B4srPsiiCi6xZ4HkvpShKoDxuBr7BoNEGic2M from node 2
[*] Retrieved file with size:  1324643
```

## Understanding the example

The example comprises the following parts:
* A [main function](./main.go#L309-L362) where all the action happens. Here we define the [size of the random file](./main.go#L312) to be generated, we [spawn two IPFS nodes](./main.go#L317-L335), we [connect both nodes](./main.go#L337-L342), we [generate a random file and added it to the network from node 1](./main.go#L344-L351) and finally [retrieve it from node2](./main.go#L353-L367).

* The nodes are spawned using the same [`NewNode` function](./main.go#L248-L307) which initializes the node and injects all the corresponding dependencies. In this example both nodes are using the same configuration.
* Nodes return their [`close` function](./main.go#L319) in case they want to [be gracefully closed](./main.go#L368-L372), as this way of spawning nodes generate a nil pointer reference error with `node.Close()` as it can't be properly initialized.

* The configuration and dependencies of the node are set in the [`setConfig` function](./main.go#L68-L249). This is the place to go if you want to change some configurations of the nodes to be spawned. In this function you will be able to do some cool stuff such as:
    * Setting the listening address for the nodes, or [allow them to listen from any available port](./main.go#L92-L98).
    * Choosing the [`routingOption` to use](./main.go#L126-L130).
    * Or setting [your custom exchange interface](./main.go#L132-L170)
    * Dependencies are injected [here](./main.go#L185-L244). Many of the ones used for this example are the default ones, but you could customize and set them at your desire as done for `hostOpotions` and the `exchangeInterface`.

* More advanced configurations coming in the future.
