package namesys

import (
	"net"

	context "golang.org/x/net/context"
	b58 "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-base58"
	isd "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	u "github.com/jbenet/go-ipfs/util"
)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// CanResolve implements Resolver
func (r *DNSResolver) CanResolve(name string) bool {
	return isd.IsDomain(name)
}

// Resolve implements Resolver
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (u.Key, error) {
	log.Info("DNSResolver resolving %v", name)
	txt, err := net.LookupTXT(name)
	if err != nil {
		return "", err
	}

	for _, t := range txt {
		chk := b58.Decode(t)
		if len(chk) == 0 {
			continue
		}

		_, err := mh.Cast(chk)
		if err != nil {
			continue
		}
		return u.Key(chk), nil
	}

	return "", ErrResolveFailed
}
