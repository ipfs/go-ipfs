package namesys

import (
	"errors"
	"net"
	"strings"

	isd "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-is-domain"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"

	path "github.com/ipfs/go-ipfs/path"
)

type LookupTXTFunc func(name string) (txt []string, err error)

// DNSResolver implements a Resolver on DNS domains
type DNSResolver struct {
	lookupTXT LookupTXTFunc
	// TODO: maybe some sort of caching?
	// cache would need a timeout
}

// NewDNSResolver constructs a name resolver using DNS TXT records.
func NewDNSResolver() Resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// newDNSResolver constructs a name resolver using DNS TXT records,
// returning a resolver instead of NewDNSResolver's Resolver.
func newDNSResolver() resolver {
	return &DNSResolver{lookupTXT: net.LookupTXT}
}

// Resolve implements Resolver.
func (r *DNSResolver) Resolve(ctx context.Context, name string) (path.Path, error) {
	return r.ResolveN(ctx, name, DefaultDepthLimit)
}

// ResolveN implements Resolver.
func (r *DNSResolver) ResolveN(ctx context.Context, name string, depth int) (path.Path, error) {
	return resolve(ctx, r, name, depth, "/ipns/")
}

// resolveOnce implements resolver.
// TXT records for a given domain name should contain a b58
// encoded multihash.
func (r *DNSResolver) resolveOnce(ctx context.Context, name string) (path.Path, error) {
	segments := strings.SplitN(name, "/", 2)

	if !isd.IsDomain(segments[0]) {
		return "", errors.New("not a valid domain name")
	}

	log.Infof("DNSResolver resolving %s", segments[0])
	txt, err := r.lookupTXT(segments[0])
	if err != nil {
		return "", err
	}

	for _, t := range txt {
		p, err := parseEntry(t)
		if err == nil {
			if len(segments) > 1 {
				return path.FromSegments("", strings.TrimRight(p.String(), "/"), segments[1])
			}
			return p, nil
		}
	}

	return "", ErrResolveFailed
}

func parseEntry(txt string) (path.Path, error) {
	parts := strings.SplitN(txt, "=", 2)
	if len(parts) == 2 && parts[0] == "dnslink" {
		return path.ParsePath(parts[1])
	}

	return "", errors.New("not a valid dnslink entry")
}
