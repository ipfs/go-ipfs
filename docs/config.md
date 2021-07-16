# The go-ipfs config file

The go-ipfs config file is a JSON document located at `$IPFS_PATH/config`. It
is read once at node instantiation, either for an offline command, or when
starting the daemon. Commands that execute on a running daemon do not read the
config file at runtime.

## Profiles

Configuration profiles allow to tweak configuration quickly. Profiles can be
applied with the `--profile` flag to `ipfs init` or with the `ipfs config profile
apply` command. When a profile is applied a backup of the configuration file
will be created in `$IPFS_PATH`.

The available configuration profiles are listed below. You can also find them
documented in `ipfs config profile --help`.

- `server`

  Disables local host discovery, recommended when
  running IPFS on machines with public IPv4 addresses.

- `randomports`

  Use a random port number for the incoming swarm connections.

- `default-datastore`

  Configures the node to use the default datastore (flatfs).

  Read the "flatfs" profile description for more information on this datastore.

  This profile may only be applied when first initializing the node.

- `local-discovery`

  Enables local discovery (enabled by default). Useful to re-enable local discovery after it's
  disabled by another profile (e.g., the server profile).

- `test`

  Reduces external interference of IPFS daemon, this
  is useful when using the daemon in test environments.

- `default-networking`

  Restores default network settings.
  Inverse profile of the test profile.

- `flatfs`

  Configures the node to use the flatfs datastore.

  This is the most battle-tested and reliable datastore, but it's significantly
  slower than the badger datastore. You should use this datastore if:

  - You need a very simple and very reliable datastore and you trust your
    filesystem. This datastore stores each block as a separate file in the
    underlying filesystem so it's unlikely to lose data unless there's an issue
    with the underlying file system.
  - You need to run garbage collection on a small (<= 10GiB) datastore. The
    default datastore, badger, can leave several gigabytes of data behind when
    garbage collecting.
  - You're concerned about memory usage. In its default configuration, badger can
    use up to several gigabytes of memory.

  This profile may only be applied when first initializing the node.


- `badgerds`

  Configures the node to use the badger datastore.

  This is the fastest datastore. Use this datastore if performance, especially
  when adding many gigabytes of files, is critical. However:

  - This datastore will not properly reclaim space when your datastore is
    smaller than several gigabytes. If you run IPFS with '--enable-gc' (you have
    enabled block-level garbage collection), you plan on storing very little data in
    your IPFS node, and disk usage is more critical than performance, consider using
    flatfs.
  - This datastore uses up to several gigabytes of memory.

  This profile may only be applied when first initializing the node.

- `lowpower`

  Reduces daemon overhead on the system. May affect node
  functionality - performance of content discovery and data
  fetching may be degraded.

## Types

This document refers to the standard JSON types (e.g., `null`, `string`,
`number`, etc.), as well as a few custom types, described below.

### `flag`

Flags allow enabling and disabling features. However, unlike simple booleans,
they can also be `null` (or omitted) to indicate that the default value should
be chosen. This makes it easier for go-ipfs to change the defaults in the
future unless the user _explicitly_ sets the flag to either `true` (enabled) or
`false` (disabled). Flags have three possible states:

- `null` or missing (apply the default value).
- `true` (enabled)
- `false` (disabled)

### `priority`

Priorities allow specifying the priority of a feature/protocol and disabling the
feature/protocol. Priorities can take one of the following values:

- `null`/missing (apply the default priority, same as with flags)
- `false` (disabled)
- `1 - 2^63` (priority, lower is preferred)

### `strings`

Strings is a special type for conveniently specifying a single string, an array
of strings, or null:

- `null`
- `"a single string"`
- `["an", "array", "of", "strings"]`

### `duration`

Duration is a type for describing lengths of time, using the same format go
does (e.g, `"1d2h4m40.01s"`).

## Table of Contents

- [`Addresses`](#addresses)
    - [`Addresses.API`](#addressesapi)
    - [`Addresses.Gateway`](#addressesgateway)
    - [`Addresses.Swarm`](#addressesswarm)
    - [`Addresses.Announce`](#addressesannounce)
    - [`Addresses.AppendAnnounce`](#addressesappendannounce)
    - [`Addresses.NoAnnounce`](#addressesnoannounce)
- [`API`](#api)
    - [`API.HTTPHeaders`](#apihttpheaders)
- [`AutoNAT`](#autonat)
    - [`AutoNAT.ServiceMode`](#autonatservicemode)
    - [`AutoNAT.Throttle`](#autonatthrottle)
    - [`AutoNAT.Throttle.GlobalLimit`](#autonatthrottlegloballimit)
    - [`AutoNAT.Throttle.PeerLimit`](#autonatthrottlepeerlimit)
    - [`AutoNAT.Throttle.Interval`](#autonatthrottleinterval)
- [`Bootstrap`](#bootstrap)
- [`Datastore`](#datastore)
    - [`Datastore.StorageMax`](#datastorestoragemax)
    - [`Datastore.StorageGCWatermark`](#datastorestoragegcwatermark)
    - [`Datastore.GCPeriod`](#datastoregcperiod)
    - [`Datastore.HashOnRead`](#datastorehashonread)
    - [`Datastore.BloomFilterSize`](#datastorebloomfiltersize)
    - [`Datastore.Spec`](#datastorespec)
- [`Discovery`](#discovery)
    - [`Discovery.MDNS`](#discoverymdns)
        - [`Discovery.MDNS.Enabled`](#discoverymdnsenabled)
        - [`Discovery.MDNS.Interval`](#discoverymdnsinterval)
- [`Gateway`](#gateway)
    - [`Gateway.NoFetch`](#gatewaynofetch)
    - [`Gateway.NoDNSLink`](#gatewaynodnslink)
    - [`Gateway.HTTPHeaders`](#gatewayhttpheaders)
    - [`Gateway.RootRedirect`](#gatewayrootredirect)
    - [`Gateway.Writable`](#gatewaywritable)
    - [`Gateway.PathPrefixes`](#gatewaypathprefixes)
    - [`Gateway.PublicGateways`](#gatewaypublicgateways)
- [`Identity`](#identity)
    - [`Identity.PeerID`](#identitypeerid)
    - [`Identity.PrivKey`](#identityprivkey)
- [`Ipns`](#ipns)
    - [`Ipns.RepublishPeriod`](#ipnsrepublishperiod)
    - [`Ipns.RecordLifetime`](#ipnsrecordlifetime)
    - [`Ipns.ResolveCacheSize`](#ipnsresolvecachesize)
- [`Migration`](#migration)
    - [`Migration.DownloadSources`](#migrationdownloadsources)
    - [`Migration.Keep`](#migrationkeep)
- [`Mounts`](#mounts)
    - [`Mounts.IPFS`](#mountsipfs)
    - [`Mounts.IPNS`](#mountsipns)
    - [`Mounts.FuseAllowOther`](#mountsfuseallowother)
- [`Pinning`](#pinning)
    - [`Pinning.RemoteServices`](#pinningremoteservices)
        - [`Pinning.RemoteServices.API`](#pinningremoteservices-api)
          - [`Pinning.RemoteServices.API.Endpoint`](#pinningremoteservices-apiendpoint)
          - [`Pinning.RemoteServices.API.Key`](#pinningremoteservices-apikey)
        - [`Pinning.RemoteServices.Policies`](#pinningremoteservices-policies)
          - [`Pinning.RemoteServices.Policies.MFS`](#pinningremoteservices-policiesmfs)
- [`Pubsub`](#pubsub)
    - [`Pubsub.Router`](#pubsubrouter)
    - [`Pubsub.DisableSigning`](#pubsubdisablesigning)
- [`Peering`](#peering)
    - [`Peering.Peers`](#peeringpeers)
- [`Reprovider`](#reprovider)
    - [`Reprovider.Interval`](#reproviderinterval)
    - [`Reprovider.Strategy`](#reproviderstrategy)
- [`Routing`](#routing)
    - [`Routing.Type`](#routingtype)
- [`Swarm`](#swarm)
    - [`Swarm.AddrFilters`](#swarmaddrfilters)
    - [`Swarm.DisableBandwidthMetrics`](#swarmdisablebandwidthmetrics)
    - [`Swarm.DisableNatPortMap`](#swarmdisablenatportmap)
    - [`Swarm.DisableRelay`](#swarmdisablerelay)
    - [`Swarm.EnableRelayHop`](#swarmenablerelayhop)
    - [`Swarm.EnableAutoRelay`](#swarmenableautorelay)
    - [`Swarm.ConnMgr`](#swarmconnmgr)
        - [`Swarm.ConnMgr.Type`](#swarmconnmgrtype)
        - [`Swarm.ConnMgr.LowWater`](#swarmconnmgrlowwater)
        - [`Swarm.ConnMgr.HighWater`](#swarmconnmgrhighwater)
        - [`Swarm.ConnMgr.GracePeriod`](#swarmconnmgrgraceperiod)
    - [`Swarm.Transports`](#swarmtransports)
        - [`Swarm.Transports.Security`](#swarmtransportssecurity)
          - [`Swarm.Transports.Security.TLS`](#swarmtransportssecuritytls)
          - [`Swarm.Transports.Security.SECIO`](#swarmtransportssecuritysecio)
          - [`Swarm.Transports.Security.Noise`](#swarmtransportssecuritynoise)
        - [`Swarm.Transports.Multiplexers`](#swarmtransportsmultiplexers)
          - [`Swarm.Transports.Multiplexers.Yamux`](#swarmtransportsmultiplexersyamux)
          - [`Swarm.Transports.Multiplexers.Mplex`](#swarmtransportsmultiplexersmplex)
        - [`Swarm.Transports.Network`](#swarmtransportsnetwork)
          - [`Swarm.Transports.Network.TCP`](#swarmtransportsnetworktcp)
          - [`Swarm.Transports.Network.QUIC`](#swarmtransportsnetworkquic)
          - [`Swarm.Transports.Network.Websocket`](#swarmtransportsnetworkwebsocket)
          - [`Swarm.Transports.Network.Relay`](#swarmtransportsnetworkrelay)
- [`DNS`](#dns)
    - [`DNS.Resolvers`](#dnsresolvers)

## `Addresses`

Contains information about various listener addresses to be used by this node.

### `Addresses.API`

Multiaddr or array of multiaddrs describing the address to serve the local HTTP
API on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/5001`

Type: `strings` (multiaddrs)

### `Addresses.Gateway`

Multiaddr or array of multiaddrs describing the address to serve the local
gateway on.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* unix - `/unix/path/to/socket`

Default: `/ip4/127.0.0.1/tcp/8080`

Type: `strings` (multiaddrs)

### `Addresses.Swarm`

An array of multiaddrs describing which addresses to listen on for p2p swarm
connections.

Supported Transports:

* tcp/ip{4,6} - `/ipN/.../tcp/...`
* websocket - `/ipN/.../tcp/.../ws`
* quic - `/ipN/.../udp/.../quic`

Default:
```json
[
  "/ip4/0.0.0.0/tcp/4001",
  "/ip6/::/tcp/4001",
  "/ip4/0.0.0.0/udp/4001/quic",
  "/ip6/::/udp/4001/quic"
]
```

Type: `array[string]` (multiaddrs)

### `Addresses.Announce`

If non-empty, this array specifies the swarm addresses to announce to the
network. If empty, the daemon will announce inferred swarm addresses.

Default: `[]`

Type: `array[string]` (multiaddrs)

### `Addresses.AppendAnnounce`

If true, the daemon will announce both [`Addresses.Announce`](#addressesannounce)
and inferred swarm addresses to the network.

Default: `false`

Type: `bool`

### `Addresses.NoAnnounce`
An array of swarm addresses not to announce to the network.

Default: `[]`

Type: `array[string]` (multiaddrs)

## `API`
Contains information used by the API gateway.

### `API.HTTPHeaders`
Map of HTTP headers to set on responses from the API HTTP server.

Example:
```json
{
	"Foo": ["bar"]
}
```

Default: `null`

Type: `object[string -> array[string]]` (header names -> array of header values)

## `AutoNAT`

Contains the configuration options for the AutoNAT service. The AutoNAT service
helps other nodes on the network determine if they're publicly reachable from
the rest of the internet.

### `AutoNAT.ServiceMode`

When unset (default), the AutoNAT service defaults to _enabled_. Otherwise, this
field can take one of two values:

* "enabled" - Enable the service (unless the node determines that it, itself,
  isn't reachable by the public internet).
* "disabled" - Disable the service.

Additional modes may be added in the future.

Type: `string` (one of `"enabled"` or `"disabled"`)

### `AutoNAT.Throttle`

When set, this option configure's the AutoNAT services throttling behavior. By
default, go-ipfs will rate-limit the number of NAT checks performed for other
nodes to 30 per minute, and 3 per peer.

### `AutoNAT.Throttle.GlobalLimit`

Configures how many AutoNAT requests to service per `AutoNAT.Throttle.Interval`.

Default: 30

Type: `integer` (non-negative, `0` means unlimited)

### `AutoNAT.Throttle.PeerLimit`

Configures how many AutoNAT requests per-peer to service per `AutoNAT.Throttle.Interval`.

Default: 3

Type: `integer` (non-negative, `0` means unlimited)

### `AutoNAT.Throttle.Interval`

Configures the interval for the above limits.

Default: 1 Minute

Type: `duration` (when `0`/unset, the default value is used)

## `Bootstrap`

Bootstrap is an array of multiaddrs of trusted nodes that your node connects to, to fetch other nodes of the network on startup.

Default: The ipfs.io bootstrap nodes

Type: `array[string]` (multiaddrs)

## `Datastore`

Contains information related to the construction and operation of the on-disk
storage system.

### `Datastore.StorageMax`

A soft upper limit for the size of the ipfs repository's datastore. With `StorageGCWatermark`,
is used to calculate whether to trigger a gc run (only if `--enable-gc` flag is set).

Default: `"10GB"`

Type: `string` (size)

### `Datastore.StorageGCWatermark`

The percentage of the `StorageMax` value at which a garbage collection will be
triggered automatically if the daemon was run with automatic gc enabled (that
option defaults to false currently).

Default: `90`

Type: `integer` (0-100%)

### `Datastore.GCPeriod`

A time duration specifying how frequently to run a garbage collection. Only used
if automatic gc is enabled.

Default: `1h`

Type: `duration` (an empty string means the default value)

### `Datastore.HashOnRead`

A boolean value. If set to true, all block reads from the disk will be hashed and
verified. This will cause increased CPU utilization.

Default: `false`

Type: `bool`

### `Datastore.BloomFilterSize`

A number representing the size in bytes of the blockstore's [bloom
filter](https://en.wikipedia.org/wiki/Bloom_filter). A value of zero represents
the feature is disabled.

This site generates useful graphs for various bloom filter values:
<https://hur.st/bloomfilter/?n=1e6&p=0.01&m=&k=7> You may use it to find a
preferred optimal value, where `m` is `BloomFilterSize` in bits. Remember to
convert the value `m` from bits, into bytes for use as `BloomFilterSize` in the
config file. For example, for 1,000,000 blocks, expecting a 1% false-positive
rate, you'd end up with a filter size of 9592955 bits, so for `BloomFilterSize`
we'd want to use 1199120 bytes. As of writing, [7 hash
functions](https://github.com/ipfs/go-ipfs-blockstore/blob/547442836ade055cc114b562a3cc193d4e57c884/caching.go#L22)
are used, so the constant `k` is 7 in the formula.

Default: `0` (disabled)

Type: `integer` (non-negative, bytes)

### `Datastore.Spec`

Spec defines the structure of the ipfs datastore. It is a composable structure,
where each datastore is represented by a json object. Datastores can wrap other
datastores to provide extra functionality (eg metrics, logging, or caching).

This can be changed manually, however, if you make any changes that require a
different on-disk structure, you will need to run the [ipfs-ds-convert
tool](https://github.com/ipfs/ipfs-ds-convert) to migrate data into the new
structures.

For more information on possible values for this configuration option, see
[docs/datastores.md](datastores.md)

Default:
```
{
  "mounts": [
	{
	  "child": {
		"path": "blocks",
		"shardFunc": "/repo/flatfs/shard/v1/next-to-last/2",
		"sync": true,
		"type": "flatfs"
	  },
	  "mountpoint": "/blocks",
	  "prefix": "flatfs.datastore",
	  "type": "measure"
	},
	{
	  "child": {
		"compression": "none",
		"path": "datastore",
		"type": "levelds"
	  },
	  "mountpoint": "/",
	  "prefix": "leveldb.datastore",
	  "type": "measure"
	}
  ],
  "type": "mount"
}
```

Type: `object`

## `Discovery`

Contains options for configuring ipfs node discovery mechanisms.

### `Discovery.MDNS`

Options for multicast dns peer discovery.

#### `Discovery.MDNS.Enabled`

A boolean value for whether or not mdns should be active.

Default: `true`

Type: `bool`

#### `Discovery.MDNS.Interval`

The number of seconds between discovery checks.

Default: `5`

Type: `integer` (integer seconds, 0 means the default)

## `Gateway`

Options for the HTTP gateway.

### `Gateway.NoFetch`

When set to true, the gateway will only serve content already in the local repo
and will not fetch files from the network.

Default: `false`

Type: `bool`

### `Gateway.NoDNSLink`

A boolean to configure whether DNSLink lookup for value in `Host` HTTP header
should be performed.  If DNSLink is present, the content path stored in the DNS TXT
record becomes the `/` and the respective payload is returned to the client.

Default: `false`

Type: `bool`

### `Gateway.HTTPHeaders`

Headers to set on gateway responses.

Default:
```json
{
	"Access-Control-Allow-Headers": [
		"X-Requested-With"
	],
	"Access-Control-Allow-Methods": [
		"GET"
	],
	"Access-Control-Allow-Origin": [
		"*"
	]
}
```

Type: `object[string -> array[string]]`

### `Gateway.RootRedirect`

A url to redirect requests for `/` to.

Default: `""`

Type: `string` (url)

### `Gateway.Writable`

A boolean to configure whether the gateway is writeable or not.

Default: `false`

Type: `bool`

### `Gateway.PathPrefixes`

**DEPRECATED:** see [go-ipfs#7702](https://github.com/ipfs/go-ipfs/issues/7702)

<!--
An array of acceptable url paths that a client can specify in X-Ipfs-Path-Prefix
header.

The X-Ipfs-Path-Prefix header is used to specify a base path to prepend to links
in directory listings and for trailing-slash redirects. It is intended to be set
by a frontend http proxy like nginx.

Example: We mount `blog.ipfs.io` (a dnslink page) at `ipfs.io/blog`.

**.ipfs/config**
```json
"Gateway": {
  "PathPrefixes": ["/blog"],
}
```

**nginx_ipfs.conf**
```nginx
location /blog/ {
  rewrite "^/blog(/.*)$" $1 break;
  proxy_set_header Host blog.ipfs.io;
  proxy_set_header X-Ipfs-Gateway-Prefix /blog;
  proxy_pass http://127.0.0.1:8080;
}
```

-->

Default: `[]`

Type: `array[string]`

### `Gateway.PublicGateways`

`PublicGateways` is a dictionary for defining gateway behavior on specified hostnames.

Hostnames can optionally be defined with one or more wildcards.

Examples:
- `*.example.com` will match requests to `http://foo.example.com/ipfs/*` or `http://{cid}.ipfs.bar.example.com/*`.
- `foo-*.example.com` will match requests to `http://foo-bar.example.com/ipfs/*` or `http://{cid}.ipfs.foo-xyz.example.com/*`.

#### `Gateway.PublicGateways: Paths`

An array of paths that should be exposed on the hostname.

Example:
```json
{
  "Gateway": {
    "PublicGateways": {
      "example.com": {
        "Paths": ["/ipfs", "/ipns"],
      }
    }
  }
}
```

Above enables `http://example.com/ipfs/*` and `http://example.com/ipns/*` but not `http://example.com/api/*`

Default: `[]`

Type: `array[string]`

#### `Gateway.PublicGateways: UseSubdomains`

A boolean to configure whether the gateway at the hostname provides [Origin isolation](https://developer.mozilla.org/en-US/docs/Web/Security/Same-origin_policy)
between content roots.

- `true` - enables [subdomain gateway](#https://docs.ipfs.io/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://*.{hostname}/`
    - **Requires whitelist:** make sure respective `Paths` are set.
    For example, `Paths: ["/ipfs", "/ipns"]` are required for `http://{cid}.ipfs.{hostname}` and `http://{foo}.ipns.{hostname}` to work:
        ```json
        "Gateway": {
            "PublicGateways": {
                "dweb.link": {
                    "UseSubdomains": true,
                    "Paths": ["/ipfs", "/ipns"],
                }
            }
        }
        ```
    - **Backward-compatible:** requests for content paths such as `http://{hostname}/ipfs/{cid}` produce redirect to `http://{cid}.ipfs.{hostname}`
    - **API:** if `/api` is on the `Paths` whitelist, `http://{hostname}/api/{cmd}` produces redirect to `http://api.{hostname}/api/{cmd}`

- `false` - enables [path gateway](https://docs.ipfs.io/how-to/address-ipfs-on-web/#path-gateway) at `http://{hostname}/*`
  - Example:
    ```json
    "Gateway": {
        "PublicGateways": {
            "ipfs.io": {
                "UseSubdomains": false,
                "Paths": ["/ipfs", "/ipns", "/api"],
            }
        }
    }
    ```

Default: `false`

Type: `bool`

#### `Gateway.PublicGateways: NoDNSLink`

A boolean to configure whether DNSLink for hostname present in `Host`
HTTP header should be resolved. Overrides global setting.
If `Paths` are defined, they take priority over DNSLink.

Default: `false` (DNSLink lookup enabled by default for every defined hostname)

Type: `bool`

#### Implicit defaults of `Gateway.PublicGateways`

Default entries for `localhost` hostname and loopback IPs are always present.
If additional config is provided for those hostnames, it will be merged on top of implicit values:
```json
{
  "Gateway": {
    "PublicGateways": {
      "localhost": {
        "Paths": ["/ipfs", "/ipns"],
        "UseSubdomains": true
      }
    }
  }
}
```

It is also possible to remove a default by setting it to `null`.

For example, to disable subdomain gateway on `localhost`
and make that hostname act the same as `127.0.0.1`:

```console
$ ipfs config --json Gateway.PublicGateways '{"localhost": null }'
```

### `Gateway` recipes

Below is a list of the most common public gateway setups.

* Public [subdomain gateway](https://docs.ipfs.io/how-to/address-ipfs-on-web/#subdomain-gateway) at `http://{cid}.ipfs.dweb.link` (each content root gets its own Origin)
   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "dweb.link": {
         "UseSubdomains": true,
         "Paths": ["/ipfs", "/ipns"]
       }
     }'
   ```
   - **Backward-compatible:** this feature enables automatic redirects from content paths to subdomains:

     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.dweb.link`

   - **X-Forwarded-Proto:** if you run go-ipfs behind a reverse proxy that provides TLS, make it add a `X-Forwarded-Proto: https` HTTP header to ensure users are redirected to `https://`, not `http://`. It will also ensure DNSLink names are inlined to fit in a single DNS label, so they work fine with a wildcart TLS cert ([details](https://github.com/ipfs/in-web-browsers/issues/169)). The NGINX directive is `proxy_set_header X-Forwarded-Proto "https";`.:

     `http://dweb.link/ipfs/{cid}` → `https://{cid}.ipfs.dweb.link`

     `http://dweb.link/ipns/your-dnslink.site.example.com` → `https://your--dnslink-site-example-com.ipfs.dweb.link`

   - **X-Forwarded-Host:** we also support `X-Forwarded-Host: example.com` if you want to override subdomain gateway host from the original request:

     `http://dweb.link/ipfs/{cid}` → `http://{cid}.ipfs.example.com`


* Public [path gateway](https://docs.ipfs.io/how-to/address-ipfs-on-web/#path-gateway) at `http://ipfs.io/ipfs/{cid}` (no Origin separation)
   ```console
   $ ipfs config --json Gateway.PublicGateways '{
       "ipfs.io": {
         "UseSubdomains": false,
         "Paths": ["/ipfs", "/ipns", "/api"]
       }
     }'
   ```

* Public [DNSLink](https://dnslink.io/) gateway resolving every hostname passed in `Host` header.
  ```console
  $ ipfs config --json Gateway.NoDNSLink false
  ```
  * Note that `NoDNSLink: false` is the default (it works out of the box unless set to `true` manually)

* Hardened, site-specific [DNSLink gateway](https://docs.ipfs.io/how-to/address-ipfs-on-web/#dnslink-gateway).

  Disable fetching of remote data (`NoFetch: true`) and resolving DNSLink at unknown hostnames (`NoDNSLink: true`).
  Then, enable DNSLink gateway only for the specific hostname (for which data
  is already present on the node), without exposing any content-addressing `Paths`:

   ```console
   $ ipfs config --json Gateway.NoFetch true
   $ ipfs config --json Gateway.NoDNSLink true
   $ ipfs config --json Gateway.PublicGateways '{
       "en.wikipedia-on-ipfs.org": {
         "NoDNSLink": false,
         "Paths": []
       }
     }'
   ```

## `Identity`

### `Identity.PeerID`

The unique PKI identity label for this configs peer. Set on init and never read,
it's merely here for convenience. Ipfs will always generate the peerID from its
keypair at runtime.

Type: `string` (peer ID)

### `Identity.PrivKey`

The base64 encoded protobuf describing (and containing) the node's private key.

Type: `string` (base64 encoded)

## `Ipns`

### `Ipns.RepublishPeriod`

A time duration specifying how frequently to republish ipns records to ensure
they stay fresh on the network.

Default: 4 hours.

Type: `interval` or an empty string for the default.

### `Ipns.RecordLifetime`

A time duration specifying the value to set on ipns records for their validity
lifetime.

Default: 24 hours.

Type: `interval` or an empty string for the default.

### `Ipns.ResolveCacheSize`

The number of entries to store in an LRU cache of resolved ipns entries. Entries
will be kept cached until their lifetime is expired.

Default: `128`

Type: `integer` (non-negative, 0 means the default)

## `Migration`

Migration configures how migrations are downloaded and if the downloads are added to IPFS locally.

### `Migration.DownloadSources`

Sources in order of preference, where "IPFS" means use IPFS and "HTTPS" means use default gateways. Any other values are interpreted as hostnames for custom gateways. An empty list means "use default sources".

Default: `["HTTPS", "IPFS"]`

### `Migration.Keep`

Specifies whether or not to keep the migration after downloading it. Options are "discard", "cache", "pin". Empty string for default.

Default: `cache`

## `Mounts`

FUSE mount point configuration options.

### `Mounts.IPFS`

Mountpoint for `/ipfs/`.

Default: `/ipfs`

Type: `string` (filesystem path)

### `Mounts.IPNS`

Mountpoint for `/ipns/`.

Default: `/ipns`

Type: `string` (filesystem path)

### `Mounts.FuseAllowOther`

Sets the 'FUSE allow other'-option on the mount point.

## `Pinning`

Pinning configures the options available for pinning content
(i.e. keeping content longer-term instead of as temporarily cached storage).

### `Pinning.RemoteServices`

`RemoteServices` maps a name for a remote pinning service to its configuration.

A remote pinning service is a remote service that exposes an API for managing
that service's interest in long-term data storage.

The exposed API conforms to the specification defined at
https://ipfs.github.io/pinning-services-api-spec/

#### `Pinning.RemoteServices: API`

Contains information relevant to utilizing the remote pinning service

Example:
```json
{
  "Pinning": {
    "RemoteServices": {
      "myPinningService": {
        "API" : {
          "Endpoint" : "https://pinningservice.tld:1234/my/api/path",
          "Key" : "someOpaqueKey"
				}
      }
    }
  }
}
```

##### `Pinning.RemoteServices: API.Endpoint`

The HTTP(S) endpoint through which to access the pinning service

Example: "https://pinningservice.tld:1234/my/api/path"

Type: `string`

##### `Pinning.RemoteServices: API.Key`

The key through which access to the pinning service is granted

Type: `string`

#### `Pinning.RemoteServices: Policies`

Contains additional opt-in policies for the remote pinning service.

##### `Pinning.RemoteServices: Policies.MFS`

When this policy is enabled, it follows changes to MFS
and updates the pin for MFS root on the configured remote service.

A pin request to the remote service is sent only when MFS root CID has changed
and enough time has passed since the previous request (determined by `RepinInterval`).

One can observe MFS pinning details by enabling debug via `ipfs log level remotepinning/mfs debug` and switching back to `error` when done.

###### `Pinning.RemoteServices: Policies.MFS.Enabled`

Controls if this policy is active.

Default: `false`

Type: `bool`

###### `Pinning.RemoteServices: Policies.MFS.PinName`

Optional name to use for a remote pin that represents the MFS root CID.
When left empty, a default name will be generated.

Default: `"policy/{PeerID}/mfs"`, e.g. `"policy/12.../mfs"`

Type: `string`

###### `Pinning.RemoteServices: Policies.MFS.RepinInterval`

Defines how often (at most) the pin request should be sent to the remote service.
If left empty, the default interval will be used. Values lower than `1m` will be ignored.

Default: `"5m"`

Type: `duration`

## `Pubsub`

Pubsub configures the `ipfs pubsub` subsystem. To use, it must be enabled by
passing the `--enable-pubsub-experiment` flag to the daemon.

### `Pubsub.Router`

Sets the default router used by pubsub to route messages to peers. This can be one of:

* `"floodsub"` - floodsub is a basic router that simply _floods_ messages to all
  connected peers. This router is extremely inefficient but _very_ reliable.
* `"gossipsub"` - [gossipsub][] is a more advanced routing algorithm that will
  build an overlay mesh from a subset of the links in the network.

Default: `"gossipsub"`

Type: `string` (one of `"floodsub"`, `"gossipsub"`, or `""` (apply default))

[gossipsub]: https://github.com/libp2p/specs/tree/master/pubsub/gossipsub

### `Pubsub.DisableSigning`

Disables message signing and signature verification. Enable this option if
you're operating in a completely trusted network.

It is _not_ safe to disable signing even if you don't care _who_ sent the
message because spoofed messages can be used to silence real messages by
intentionally re-using the real message's message ID.

Default: `false`

Type: `bool`

## `Peering`

Configures the peering subsystem. The peering subsystem configures go-ipfs to
connect to, remain connected to, and reconnect to a set of nodes. Nodes should
use this subsystem to create "sticky" links between frequently useful peers to
improve reliability.

Use-cases:

* An IPFS gateway connected to an IPFS cluster should peer to ensure that the
  gateway can always fetch content from the cluster.
* A dapp may peer embedded go-ipfs nodes with a set of pinning services or
  textile cafes/hubs.
* A set of friends may peer to ensure that they can always fetch each other's
  content.

When a node is added to the set of peered nodes, go-ipfs will:

1. Protect connections to this node from the connection manager. That is,
   go-ipfs will never automatically close the connection to this node and
   connections to this node will not count towards the connection limit.
2. Connect to this node on startup.
3. Repeatedly try to reconnect to this node if the last connection dies or the
   node goes offline. This repeated re-connect logic is governed by a randomized
   exponential backoff delay ranging from ~5 seconds to ~10 minutes to avoid
   repeatedly reconnect to a node that's offline.

Peering can be asymmetric or symmetric:

* When symmetric, the connection will be protected by both nodes and will likely
  be very stable.
* When asymmetric, only one node (the node that configured peering) will protect
  the connection and attempt to re-connect to the peered node on disconnect. If
  the peered node is under heavy load and/or has a low connection limit, the
  connection may flap repeatedly. Be careful when asymmetrically peering to not
  overload peers.

### `Peering.Peers`

The set of peers with which to peer.

```json
{
  "Peering": {
    "Peers": [
      {
        "ID": "QmPeerID1",
        "Addrs": ["/ip4/18.1.1.1/tcp/4001"]
      },
      {
        "ID": "QmPeerID2",
        "Addrs": ["/ip4/18.1.1.2/tcp/4001", "/ip4/18.1.1.2/udp/4001/quic"]
      }
    ]
  }
  ...
}
```

Where `ID` is the peer ID and `Addrs` is a set of known addresses for the peer. If no addresses are specified, the DHT will be queried.

Additional fields may be added in the future.

Default: empty.

Type: `array[peering]`

## `Reprovider`

### `Reprovider.Interval`

Sets the time between rounds of reproviding local content to the routing
system. If unset, it defaults to 12 hours. If set to the value `"0"` it will
disable content reproviding.

Note: disabling content reproviding will result in other nodes on the network
not being able to discover that you have the objects that you have. If you want
to have this disabled and keep the network aware of what you have, you must
manually announce your content periodically.

Type: `array[peering]`

### `Reprovider.Strategy`

Tells reprovider what should be announced. Valid strategies are:
  - "all" - announce all stored data
  - "pinned" - only announce pinned data
  - "roots" - only announce directly pinned keys and root keys of recursive pins

Default: all

Type: `string` (or unset for the default)

## `Routing`

Contains options for content, peer, and IPNS routing mechanisms.

### `Routing.Type`

Content routing mode. Can be overridden with daemon `--routing` flag.

There are two core routing options: "none" and "dht" (default).

* If set to "none", your node will use _no_ routing system. You'll have to
  explicitly connect to peers that have the content you're looking for.
* If set to "dht" (or "dhtclient"/"dhtserver"), your node will use the IPFS DHT.

When the DHT is enabled, it can operate in two modes: client and server.

* In server mode, your node will query other peers for DHT records, and will
  respond to requests from other peers (both requests to store records and
  requests to retrieve records).
* In client mode, your node will query the DHT as a client but will not respond
  to requests from other peers. This mode is less resource-intensive than server
  mode.

When `Routing.Type` is set to `dht`, your node will start as a DHT client, and
switch to a DHT server when and if it determines that it's reachable from the
public internet (e.g., it's not behind a firewall).

To force a specific DHT mode, client or server, set `Routing.Type` to
`dhtclient` or `dhtserver` respectively. Please do not set this to `dhtserver`
unless you're sure your node is reachable from the public network.

**Example:**

```json
{
  "Routing": {
    "Type": "dhtclient"
  }
}
```

Default: dht

Type: `string` (or unset for the default)

## `Swarm`

Options for configuring the swarm.

### `Swarm.AddrFilters`

An array of addresses (multiaddr netmasks) to not dial. By default, IPFS nodes
advertise _all_ addresses, even internal ones. This makes it easier for nodes on
the same network to reach each other. Unfortunately, this means that an IPFS
node will try to connect to one or more private IP addresses whenever dialing
another node, even if this other node is on a different network. This may
trigger netscan alerts on some hosting providers or cause strain in some setups.

The `server` configuration profile fills up this list with sensible defaults,
preventing dials to all non-routable IP addresses (e.g., `192.168.0.0/16`) but
you should always check settings against your own network and/or hosting
provider.

Default: `[]`

Type: `array[string]`

### `Swarm.DisableBandwidthMetrics`

A boolean value that when set to true, will cause ipfs to not keep track of
bandwidth metrics. Disabling bandwidth metrics can lead to a slight performance
improvement, as well as a reduction in memory usage.

Default: `false`

Type: `bool`

### `Swarm.DisableNatPortMap`

Disable automatic NAT port forwarding.

When not disabled (default), go-ipfs asks NAT devices (e.g., routers), to open
up an external port and forward it to the port go-ipfs is running on. When this
works (i.e., when your router supports NAT port forwarding), it makes the local
go-ipfs node accessible from the public internet.

Default: `false`

Type: `bool`

### `Swarm.DisableRelay`

Deprecated: Set `Swarm.Transports.Network.Relay` to `false`.

Disables the p2p-circuit relay transport. This will prevent this node from
connecting to nodes behind relays, or accepting connections from nodes behind
relays.

Default: `false`

Type: `bool`

### `Swarm.EnableRelayHop`

Configures this node to act as a relay "hop". A relay "hop" relays traffic for other peers.

WARNING: Do not enable this option unless you know what you're doing. Other
peers will randomly decide to use your node as a relay and consume _all_
available bandwidth. There is _no_ rate-limiting.

Default: `false`

Type: `bool`

### `Swarm.EnableAutoRelay`

Enables "automatic relay" mode for this node. This option does two _very_
different things based on the `Swarm.EnableRelayHop`. See
[#7228](https://github.com/ipfs/go-ipfs/issues/7228) for context.

Default: `false`

Type: `bool`

#### Mode 1: `EnableRelayHop` is `false`

If `Swarm.EnableAutoRelay` is enabled and `Swarm.EnableRelayHop` is disabled,
your node will automatically _use_ public relays from the network if it detects
that it cannot be reached from the public internet (e.g., it's behind a
firewall). This is likely the feature you're looking for.

If you enable `EnableAutoRelay`, you should almost certainly disable
`EnableRelayHop`.

#### Mode 2: `EnableRelayHop` is `true`

If `EnableAutoRelay` is enabled and `EnableRelayHop` is enabled, your node will
_act_ as a public relay for the network. Furthermore, in addition to simply
relaying traffic, your node will advertise itself as a public relay. Unless you
have the bandwidth of a small ISP, do not enable both of these options at the
same time.

### `Swarm.EnableAutoNATService`

**REMOVED**

Please use [`AutoNAT.ServiceMode`][].

### `Swarm.ConnMgr`

The connection manager determines which and how many connections to keep and can
be configured to keep. Go-ipfs currently supports two connection managers:

* none: never close idle connections.
* basic: the default connection manager.

Default: basic

#### `Swarm.ConnMgr.Type`

Sets the type of connection manager to use, options are: `"none"` (no connection
management) and `"basic"`.

Default: "basic".

Type: `string` (when unset or `""`, the default connection manager is applied
and all `ConnMgr` fields are ignored).

#### Basic Connection Manager

The basic connection manager uses a "high water", a "low water", and internal
scoring to periodically close connections to free up resources. When a node
using the basic connection manager reaches `HighWater` idle connections, it will
close the least useful ones until it reaches `LowWater` idle connections.

The connection manager considers a connection idle if:

* It has not been explicitly _protected_ by some subsystem. For example, Bitswap
  will protect connections to peers from which it is actively downloading data,
  the DHT will protect some peers for routing, and the peering subsystem will
  protect all "peered" nodes.
* It has existed for longer than the `GracePeriod`.

**Example:**

```json
{
  "Swarm": {
    "ConnMgr": {
      "Type": "basic",
      "LowWater": 100,
      "HighWater": 200,
      "GracePeriod": "30s"
    }
  }
}
```

##### `Swarm.ConnMgr.LowWater`

LowWater is the number of connections that the basic connection manager will
trim down to.

Default: `600`

Type: `integer`

##### `Swarm.ConnMgr.HighWater`

HighWater is the number of connections that, when exceeded, will trigger a
connection GC operation. Note: protected/recently formed connections don't count
towards this limit.

Default: `900`

Type: `integer`

##### `Swarm.ConnMgr.GracePeriod`

GracePeriod is a time duration that new connections are immune from being closed
by the connection manager.

Default: `"20s"`

Type: `duration`

### `Swarm.Transports`

Configuration section for libp2p transports. An empty configuration will apply
the defaults.

### `Swarm.Transports.Network`

Configuration section for libp2p _network_ transports. Transports enabled in
this section will be used for dialing. However, to receive connections on these
transports, multiaddrs for these transports must be added to `Addresses.Swarm`.

Supported transports are: QUIC, TCP, WS, and Relay.

Each field in this section is a `flag`.

#### `Swarm.Transports.Network.TCP`

[TCP](https://en.wikipedia.org/wiki/Transmission_Control_Protocol) is the most
widely used transport by go-ipfs nodes. It doesn't directly support encryption
and/or multiplexing, so libp2p will layer a security & multiplexing transport
over it.

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/tcp/4001 (default)
* /ip6/::/tcp/4001 (default)

#### `Swarm.Transports.Network.Websocket`

[Websocket](https://en.wikipedia.org/wiki/WebSocket) is a transport usually used
to connect to non-browser-based IPFS nodes from browser-based js-ipfs nodes.

While it's enabled by default for dialing, go-ipfs doesn't listen on this
transport by default.

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/tcp/4002/ws
* /ip6/::/tcp/4002/ws

#### `Swarm.Transports.Network.QUIC`

[QUIC](https://en.wikipedia.org/wiki/QUIC) is a UDP-based transport with
built-in encryption and multiplexing. The primary benefits over TCP are:

1. It doesn't require a file descriptor per connection, easing the load on the OS.
2. It currently takes 2 round trips to establish a connection (our TCP transport
   currently takes 6).

Default: Enabled

Type: `flag`

Listen Addresses:
* /ip4/0.0.0.0/udp/4001/quic (default)
* /ip6/::/udp/4001/quic (default)

#### `Swarm.Transports.Network.Relay`

[Libp2p Relay](https://github.com/libp2p/specs/tree/master/relay) proxy
transport that forms connections by hopping between multiple libp2p nodes. This
transport is primarily useful for bypassing firewalls and NATs.

Default: Enabled

Type: `flag`

Listen Addresses: This transport is special. Any node that enables this
transport can receive inbound connections on this transport, without specifying
a listen address.

### `Swarm.Transports.Security`

Configuration section for libp2p _security_ transports. Transports enabled in
this section will be used to secure unencrypted connections.

Security transports are configured with the `priority` type.

When establishing an _outbound_ connection, go-ipfs will try each security
transport in priority order (lower first), until it finds a protocol that the
receiver supports. When establishing an _inbound_ connection, go-ipfs will let
the initiator choose the protocol, but will refuse to use any of the disabled
transports.

Supported transports are: TLS (priority 100) and Noise (priority 300).

No default priority will ever be less than 100.

#### `Swarm.Transports.Security.TLS`

[TLS](https://github.com/libp2p/specs/tree/master/tls) (1.3) is the default
security transport as of go-ipfs 0.5.0. It's also the most scrutinized and
trusted security transport.

Default: `100`

Type: `priority`

#### `Swarm.Transports.Security.SECIO`

Support for SECIO has been removed. Please remove this option from your config.

#### `Swarm.Transports.Security.Noise`

[Noise](https://github.com/libp2p/specs/tree/master/noise) is slated to replace
TLS as the cross-platform, default libp2p protocol due to ease of
implementation. It is currently enabled by default but with low priority as it's
not yet widely supported.

Default: `300`

Type: `priority`

### `Swarm.Transports.Multiplexers`

Configuration section for libp2p _multiplexer_ transports. Transports enabled in
this section will be used to multiplex duplex connections.

Multiplexer transports are secured the same way security transports are, with
the `priority` type. Like with security transports, the initiator gets their
first choice.

Supported transports are: Yamux (priority 100) and Mplex (priority 200)

No default priority will ever be less than 100.

### `Swarm.Transports.Multiplexers.Yamux`

Yamux is the default multiplexer used when communicating between go-ipfs nodes.

Default: `100`

Type: `priority`

### `Swarm.Transports.Multiplexers.Mplex`

Mplex is the default multiplexer used when communicating between go-ipfs and all
other IPFS and libp2p implementations. Unlike Yamux:

* Mplex is a simpler protocol.
* Mplex is more efficient.
* Mplex does not have built-in keepalives.
* Mplex does not support backpressure. Unfortunately, this means that, if a
  single stream to a peer gets backed up for a period of time, the mplex
  transport will kill the stream to allow the others to proceed. On the other
  hand, the lack of backpressure means mplex can be significantly faster on some
  high-latency connections.

Default: `200`

Type: `priority`

## `DNS`

Options for configuring DNS resolution for [DNSLink](https://docs.ipfs.io/concepts/dnslink/) and `/dns*` [Multiaddrs](https://github.com/multiformats/multiaddr/).

## `DNS.Resolvers`

Map of [FQDNs](https://en.wikipedia.org/wiki/Fully_qualified_domain_name) to custom resolver URLs.

This allows for overriding the default DNS resolver provided by the operating system,
and using different resolvers per domain or TLD (including ones from alternative, non-ICANN naming systems).

Example:
```json
{
  "DNS": {
    "Resolvers": {
      "eth.": "https://eth.link/dns-query",
      "crypto.": "https://resolver.unstoppable.io/dns-query",
      "libre.": "https://ns1.iriseden.fr/dns-query",
      ".": "https://doh-ch.blahdns.com:4443/dns-query"
    }
  }
}
```

Be mindful that:
- Currently only `https://` URLs for [DNS over HTTPS (DoH)](https://en.wikipedia.org/wiki/DNS_over_HTTPS) endpoints are supported as values.
- The default catch-all resolver is the cleartext one provided by your operating system. It can be overriden by adding a DoH entry for the DNS root indicated by  `.` as illustrated above.
- Out-of-the-box support for selected decentralized TLDs relies on a [centralized service which is provided on best-effort basis](https://www.cloudflare.com/distributed-web-gateway-terms/). The implicit DoH resolvers are:
  ```json
  {
    "eth.": "https://resolver.cloudflare-eth.com/dns-query",
    "crypto.": "https://resolver.cloudflare-eth.com/dns-query
  }
  ```
  To get all the benefits of a decentralized naming system we strongly suggest setting DoH endpoint to an empty string and running own decentralized resolver as catch-all one on localhost.

Default: `{}`

Type: `object[string -> string]`
