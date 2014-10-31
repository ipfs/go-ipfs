package http

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/gorilla/mux"
	ma "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr"
	manet "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multiaddr/net"
	mh "github.com/jbenet/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-multihash"

	core "github.com/jbenet/go-ipfs/core"
)

type objectHandler struct {
	ipfs
}

// Serve starts the http server
func Serve(address ma.Multiaddr, node *core.IpfsNode) error {
	r := mux.NewRouter()
	objectHandler := &objectHandler{&ipfsHandler{node}}
	apiHandler := &apiHandler{}

	r.HandleFunc("/ipfs/", objectHandler.postHandler).Methods("POST")
	r.PathPrefix("/ipfs/").Handler(objectHandler).Methods("GET")

	http.Handle("/", r)

	_, host, err := manet.DialArgs(address)
	if err != nil {
		return err
	}

	return http.ListenAndServe(host, nil)
}

func (i *objectHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[5:]

	nd, err := i.ResolvePath(path)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	dr, err := i.NewDagReader(nd)
	if err != nil {
		// TODO: return json object containing the tree data if it's a directory (err == ErrIsDir)
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	io.Copy(w, dr)
}

func (i *objectHandler) postHandler(w http.ResponseWriter, r *http.Request) {
	nd, err := i.NewDagFromReader(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	k, err := i.AddNodeToDAG(nd)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Println(err)
		return
	}

	//TODO: return json representation of list instead
	w.WriteHeader(http.StatusCreated)
	w.Write([]byte(mh.Multihash(k).B58String()))
}
