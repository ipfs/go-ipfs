// +build !nofuse

package node

import (
	"github.com/ipfs/go-ipfs/core/coreapi"
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	"bazil.org/fuse"

	"context"

	"github.com/ipfs/go-ipfs/fuse/ipns"
	"github.com/ipfs/go-ipfs/fuse/mount"

	ci "github.com/libp2p/go-libp2p-testing/ci"
)

func maybeSkipFuseTests(t *testing.T) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}
}

func mkdir(t *testing.T, path string) {
	err := os.Mkdir(path, os.ModeDir|os.ModePerm)
	if err != nil {
		t.Fatal(err)
	}
}

// Test externally unmounting, then trying to unmount in code
func TestExternalUnmount(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// TODO: needed?
	maybeSkipFuseTests(t)

	api, err := coreapi.New(
		coreapi.Ctx(context.Background()),
		coreapi.RandomIdentity(),
	)
	if err != nil {
		t.Fatal(err)
	}
	// nolint
	node := api.Node()

	err = ipns.InitializeKeyspace(node, node.PrivateKey)
	if err != nil {
		t.Fatal(err)
	}

	// get the test dir paths (/tmp/fusetestXXXX)
	dir, err := ioutil.TempDir("", "fusetest")
	if err != nil {
		t.Fatal(err)
	}

	ipfsDir := dir + "/ipfs"
	ipnsDir := dir + "/ipns"
	mkdir(t, ipfsDir)
	mkdir(t, ipnsDir)

	err = Mount(node, ipfsDir, ipnsDir)
	if err != nil {
		if strings.Contains(err.Error(), "unable to check fuse version") || err == fuse.ErrOSXFUSENotFound {
			t.Skip(err)
		}
	}

	if err != nil {
		t.Fatalf("error mounting: %v", err)
	}

	// Run shell command to externally unmount the directory
	cmd, err := mount.UnmountCmd(ipfsDir)
	if err != nil {
		t.Fatal(err)
	}

	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	// TODO(noffle): it takes a moment for the goroutine that's running fs.Serve to be notified and do its cleanup.
	time.Sleep(time.Millisecond * 100)

	// Attempt to unmount IPFS; it should unmount successfully.
	err = node.Mounts.Ipfs.Unmount()
	if err != mount.ErrNotMounted {
		t.Fatal("Unmount should have failed")
	}
}
