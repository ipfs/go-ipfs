/*
Package corehttp provides utilities for the webui, gateways, and other
high-level HTTP interfaces to IPFS.
*/
package corehttp

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"

	core "github.com/ipfs/go-ipfs/core"

	logging "github.com/ipfs/go-log"
	"github.com/jbenet/goprocess"
	periodicproc "github.com/jbenet/goprocess/periodic"
	ma "github.com/multiformats/go-multiaddr"
	manet "github.com/multiformats/go-multiaddr-net"
)

var log = logging.Logger("core/server")

// shutdownTimeout is the timeout after which we'll stop waiting for hung
// commands to return on shutdown.
const shutdownTimeout = 30 * time.Second

// ServeOption registers any HTTP handlers it provides on the given mux.
// It returns the mux to expose to future options, which may be a new mux if it
// is interested in mediating requests to future options, or the same mux
// initially passed in if not.
type ServeOption func(*core.IpfsNode, net.Listener, *http.ServeMux) (*http.ServeMux, error)

// DefaultMux tells mux to serve path using the default muxer. This is
// mostly useful to hook up things that register in the default muxer,
// and don't provide a convenient http.Handler entry point, such as
// expvar and http/pprof.
func DefaultMux(path string) ServeOption {
	return func(node *core.IpfsNode, _ net.Listener, mux *http.ServeMux) (*http.ServeMux, error) {
		mux.Handle(path, http.DefaultServeMux)
		return mux, nil
	}
}

// makeHandler turns a list of ServeOptions into a http.Handler that implements
// all of the given options, in order.
func makeHandler(n *core.IpfsNode, l net.Listener, options ...ServeOption) (http.Handler, error) {
	topMux := http.NewServeMux()
	mux := topMux
	for _, option := range options {
		var err error
		mux, err = option(n, l, mux)
		if err != nil {
			return nil, err
		}
	}
	return topMux, nil
}

// ListenAndServe runs an HTTP server listening at |listeningMultiAddr| with
// the given serve options. The address must be provided in multiaddr format.
//
// TODO intelligently parse address strings in other formats so long as they
// unambiguously map to a valid multiaddr. e.g. for convenience, ":8080" should
// map to "/ip4/0.0.0.0/tcp/8080".
func ListenAndServe(n *core.IpfsNode, listeningMultiAddr string, options ...ServeOption) error {
	addr, err := ma.NewMultiaddr(listeningMultiAddr)
	if err != nil {
		return err
	}

	list, err := manet.Listen(addr)
	if err != nil {
		return err
	}

	// we might have listened to /tcp/0 - lets see what we are listing on
	addr = list.Multiaddr()
	fmt.Printf("API server listening on %s\n", addr)

	return Serve(n, manet.NetListener(list), options...)
}

func Serve(node *core.IpfsNode, lis net.Listener, options ...ServeOption) error {
	// make sure we close this no matter what.
	defer lis.Close()

	handler, err := makeHandler(node, lis, options...)
	if err != nil {
		return err
	}

	addr, err := manet.FromNetAddr(lis.Addr())
	if err != nil {
		return err
	}

	select {
	case <-node.Process.Closing():
		return fmt.Errorf("failed to start server, process closing")
	default:
	}

	server := &http.Server{
		Handler: handler,
	}

	var serverError error
	serverProc := node.Process.Go(func(p goprocess.Process) {
		serverError = server.Serve(lis)
	})

	// wait for server to exit.
	select {
	case <-serverProc.Closed():
	// if node being closed before server exits, close server
	case <-node.Process.Closing():
		log.Infof("server at %s terminating...", addr)

		warnProc := periodicproc.Tick(5*time.Second, func(_ goprocess.Process) {
			log.Infof("waiting for server at %s to terminate...", addr)
		})

		// This timeout shouldn't be necessary if all of our commands
		// are obeying their contexts but we should have *some* timeout.
		ctx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
		defer cancel()
		err := server.Shutdown(ctx)

		// Should have already closed but we still need to wait for it
		// to set the error.
		<-serverProc.Closed()
		serverError = err

		warnProc.Close()
	}

	log.Infof("server at %s terminated", addr)
	return serverError
}
