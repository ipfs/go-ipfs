package libp2p

import (
	"fmt"

	"github.com/libp2p/go-libp2p"
	host "github.com/libp2p/go-libp2p-core/host"
	p2pbhost "github.com/libp2p/go-libp2p/p2p/host/basic"
	ma "github.com/multiformats/go-multiaddr"
	mamask "github.com/whyrusleeping/multiaddr-filter"
)

func AddrFilters(filters []string) func() (*ma.Filters, Libp2pOpts, error) {
	return func() (filter *ma.Filters, opts Libp2pOpts, err error) {
		filter = ma.NewFilters()
		opts.Opts = append(opts.Opts, libp2p.Filters(filter)) //nolint
		for _, s := range filters {
			f, err := mamask.NewMask(s)
			if err != nil {
				return filter, opts, fmt.Errorf("incorrectly formatted address filter in config: %s", s)
			}
			opts.Opts = append(opts.Opts, libp2p.FilterAddresses(f)) //nolint
		}
		return filter, opts, nil
	}
}

func makeAddrsFactory(announce []string, appendAnnouce bool, noAnnounce []string) (p2pbhost.AddrsFactory, error) {
	var annAddrs []ma.Multiaddr
	for _, addr := range announce {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		annAddrs = append(annAddrs, maddr)
	}

	filters := ma.NewFilters()
	noAnnAddrs := map[string]bool{}
	for _, addr := range noAnnounce {
		f, err := mamask.NewMask(addr)
		if err == nil {
			filters.AddFilter(*f, ma.ActionDeny)
			continue
		}
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, err
		}
		noAnnAddrs[string(maddr.Bytes())] = true
	}

	return func(allAddrs []ma.Multiaddr) []ma.Multiaddr {
		var addrs []ma.Multiaddr
		switch {
		case appendAnnouce:
			addrs = append(allAddrs, annAddrs...)
		case len(annAddrs) > 0:
			addrs = annAddrs
		default:
			addrs = allAddrs
		}

		var out []ma.Multiaddr
		for _, maddr := range addrs {
			// check for exact matches
			ok := noAnnAddrs[string(maddr.Bytes())]
			// check for /ipcidr matches
			if !ok && !filters.AddrBlocked(maddr) {
				out = append(out, maddr)
			}
		}
		return out
	}, nil
}

func AddrsFactory(announce []string, appendAnnouce bool, noAnnounce []string) func() (opts Libp2pOpts, err error) {
	return func() (opts Libp2pOpts, err error) {
		addrsFactory, err := makeAddrsFactory(announce, appendAnnouce, noAnnounce)
		if err != nil {
			return opts, err
		}
		opts.Opts = append(opts.Opts, libp2p.AddrsFactory(addrsFactory))
		return
	}
}

func listenAddresses(addresses []string) ([]ma.Multiaddr, error) {
	var listen []ma.Multiaddr
	for _, addr := range addresses {
		maddr, err := ma.NewMultiaddr(addr)
		if err != nil {
			return nil, fmt.Errorf("failure to parse config.Addresses.Swarm: %s", addresses)
		}
		listen = append(listen, maddr)
	}

	return listen, nil
}

func StartListening(addresses []string) func(host host.Host) error {
	return func(host host.Host) error {
		listenAddrs, err := listenAddresses(addresses)
		if err != nil {
			return err
		}

		// Actually start listening:
		if err := host.Network().Listen(listenAddrs...); err != nil {
			return err
		}

		// list out our addresses
		addrs, err := host.Network().InterfaceListenAddresses()
		if err != nil {
			return err
		}
		log.Infof("Swarm listening at: %s", addrs)
		return nil
	}
}
