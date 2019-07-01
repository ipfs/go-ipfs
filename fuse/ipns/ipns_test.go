// +build !nofuse

package ipns

import (
	"bytes"
	"context"
	"fmt"
	"github.com/ipfs/go-ipfs/core/coreapi"
	"io"
	"io/ioutil"
	mrand "math/rand"
	"os"
	"sync"
	"testing"

	"bazil.org/fuse"

	core "github.com/ipfs/go-ipfs/core"

	fstest "bazil.org/fuse/fs/fstestutil"
	racedet "github.com/ipfs/go-detect-race"
	u "github.com/ipfs/go-ipfs-util"
	ci "github.com/libp2p/go-libp2p-testing/ci"
)

func maybeSkipFuseTests(t *testing.T) {
	if ci.NoFuse() {
		t.Skip("Skipping FUSE tests")
	}
}

func randBytes(size int) []byte {
	b := make([]byte, size)
	_, err := io.ReadFull(u.NewTimeSeededRand(), b)
	if err != nil {
		panic(err)
	}
	return b
}

func mkdir(t *testing.T, path string) {
	err := os.Mkdir(path, os.ModeDir)
	if err != nil {
		t.Fatal(err)
	}
}

func writeFileOrFail(t *testing.T, size int, path string) []byte {
	data, err := writeFile(size, path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func writeFile(size int, path string) ([]byte, error) {
	data := randBytes(size)
	err := ioutil.WriteFile(path, data, 0666)
	return data, err
}

func verifyFile(t *testing.T, path string, wantData []byte) {
	isData, err := ioutil.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(isData) != len(wantData) {
		t.Fatal("Data not equal - length check failed")
	}
	if !bytes.Equal(isData, wantData) {
		t.Fatal("Data not equal")
	}
}

func checkExists(t *testing.T, path string) {
	_, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
}

func closeMount(mnt *mountWrap) {
	if err := recover(); err != nil {
		log.Error("Recovered panic")
		log.Error(err)
	}
	mnt.Close()
}

type mountWrap struct {
	*fstest.Mount
	Fs *FileSystem
}

func (m *mountWrap) Close() error {
	m.Fs.Destroy()
	m.Mount.Close()
	return nil
}

func setupIpnsTest(t *testing.T, node *core.IpfsNode) (*core.IpfsNode, *mountWrap) {
	t.Helper()
	maybeSkipFuseTests(t)

	var err error
	if node == nil {
		api, err := coreapi.New(
			coreapi.Ctx(context.Background()),
			coreapi.RandomIdentity(),
		)
		if err != nil {
			t.Fatal(err)
		}
		// nolint
		node = api.Node()

		err = InitializeKeyspace(node, node.PrivateKey)
		if err != nil {
			t.Fatal(err)
		}
	}

	fs, err := NewFileSystem(node, node.PrivateKey, "", "")
	if err != nil {
		t.Fatal(err)
	}
	mnt, err := fstest.MountedT(t, fs, nil)
	if err == fuse.ErrOSXFUSENotFound {
		t.Skip(err)
	}
	if err != nil {
		t.Fatalf("error mounting at temporary directory: %v", err)
	}

	return node, &mountWrap{
		Mount: mnt,
		Fs:    fs,
	}
}

func TestIpnsLocalLink(t *testing.T) {
	nd, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()
	name := mnt.Dir + "/local"

	checkExists(t, name)

	linksto, err := os.Readlink(name)
	if err != nil {
		t.Fatal(err)
	}

	if linksto != nd.Identity.Pretty() {
		t.Fatal("Link invalid")
	}
}

// Test writing a file and reading it back
func TestIpnsBasicIO(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	nd, mnt := setupIpnsTest(t, nil)
	defer closeMount(mnt)

	fname := mnt.Dir + "/local/testfile"
	data := writeFileOrFail(t, 10, fname)

	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}

	fname2 := mnt.Dir + "/" + nd.Identity.Pretty() + "/testfile"
	rbuf, err = ioutil.ReadFile(fname2)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("Incorrect Read!")
	}
}

// Test to make sure file changes persist over mounts of ipns
func TestFilePersistence(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	node, mnt := setupIpnsTest(t, nil)

	fname := "/local/atestfile"
	data := writeFileOrFail(t, 127, mnt.Dir+fname)

	mnt.Close()

	t.Log("Closed, opening new fs")
	_, mnt = setupIpnsTest(t, node)
	defer mnt.Close()

	rbuf, err := ioutil.ReadFile(mnt.Dir + fname)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatalf("File data changed between mounts! sizes differ: %d != %d", len(data), len(rbuf))
	}
}

func TestMultipleDirs(t *testing.T) {
	node, mnt := setupIpnsTest(t, nil)

	t.Log("make a top level dir")
	dir1 := "/local/test1"
	mkdir(t, mnt.Dir+dir1)

	checkExists(t, mnt.Dir+dir1)

	t.Log("write a file in it")
	data1 := writeFileOrFail(t, 4000, mnt.Dir+dir1+"/file1")

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	t.Log("sub directory")
	mkdir(t, mnt.Dir+dir1+"/dir2")

	checkExists(t, mnt.Dir+dir1+"/dir2")

	t.Log("file in that subdirectory")
	data2 := writeFileOrFail(t, 5000, mnt.Dir+dir1+"/dir2/file2")

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)

	mnt.Close()
	t.Log("closing mount, then restarting")

	_, mnt = setupIpnsTest(t, node)

	checkExists(t, mnt.Dir+dir1)

	verifyFile(t, mnt.Dir+dir1+"/file1", data1)

	verifyFile(t, mnt.Dir+dir1+"/dir2/file2", data2)
	mnt.Close()
}

// Test to make sure the filesystem reports file sizes correctly
func TestFileSizeReporting(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	fname := mnt.Dir + "/local/sizecheck"
	data := writeFileOrFail(t, 5555, fname)

	finfo, err := os.Stat(fname)
	if err != nil {
		t.Fatal(err)
	}

	if finfo.Size() != int64(len(data)) {
		t.Fatal("Read incorrect size from stat!")
	}
}

// Test to make sure you cant create multiple entries with the same name
func TestDoubleEntryFailure(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	dname := mnt.Dir + "/local/thisisadir"
	err := os.Mkdir(dname, 0777)
	if err != nil {
		t.Fatal(err)
	}

	err = os.Mkdir(dname, 0777)
	if err == nil {
		t.Fatal("Should have gotten error one creating new directory.")
	}
}

func TestAppendFile(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	fname := mnt.Dir + "/local/file"
	data := writeFileOrFail(t, 1300, fname)

	fi, err := os.OpenFile(fname, os.O_RDWR|os.O_APPEND, 0666)
	if err != nil {
		t.Fatal(err)
	}

	nudata := randBytes(500)

	n, err := fi.Write(nudata)
	if err != nil {
		t.Fatal(err)
	}
	err = fi.Close()
	if err != nil {
		t.Fatal(err)
	}

	if n != len(nudata) {
		t.Fatal("Failed to write enough bytes.")
	}

	data = append(data, nudata...)

	rbuf, err := ioutil.ReadFile(fname)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(rbuf, data) {
		t.Fatal("Data inconsistent!")
	}
}

func TestConcurrentWrites(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	nactors := 4
	filesPerActor := 400
	fileSize := 2000

	data := make([][][]byte, nactors)

	if racedet.WithRace() {
		nactors = 2
		filesPerActor = 50
	}

	wg := sync.WaitGroup{}
	for i := 0; i < nactors; i++ {
		data[i] = make([][]byte, filesPerActor)
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < filesPerActor; j++ {
				out, err := writeFile(fileSize, mnt.Dir+fmt.Sprintf("/local/%dFILE%d", n, j))
				if err != nil {
					t.Error(err)
					continue
				}
				data[n][j] = out
			}
		}(i)
	}
	wg.Wait()

	for i := 0; i < nactors; i++ {
		for j := 0; j < filesPerActor; j++ {
			if data[i][j] == nil {
				// Error already reported.
				continue
			}
			verifyFile(t, mnt.Dir+fmt.Sprintf("/local/%dFILE%d", i, j), data[i][j])
		}
	}
}

func TestFSThrash(t *testing.T) {
	files := make(map[string][]byte)

	if testing.Short() {
		t.SkipNow()
	}
	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	base := mnt.Dir + "/local"
	dirs := []string{base}
	dirlock := sync.RWMutex{}
	filelock := sync.Mutex{}

	ndirWorkers := 2
	nfileWorkers := 2

	ndirs := 100
	nfiles := 200

	wg := sync.WaitGroup{}

	// Spawn off workers to make directories
	for i := 0; i < ndirWorkers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < ndirs; j++ {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newDir := fmt.Sprintf("%s/dir%d-%d", dir, worker, j)
				err := os.Mkdir(newDir, os.ModeDir)
				if err != nil {
					t.Error(err)
					continue
				}
				dirlock.Lock()
				dirs = append(dirs, newDir)
				dirlock.Unlock()
			}
		}(i)
	}

	// Spawn off workers to make files
	for i := 0; i < nfileWorkers; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < nfiles; j++ {
				dirlock.RLock()
				n := mrand.Intn(len(dirs))
				dir := dirs[n]
				dirlock.RUnlock()

				newFileName := fmt.Sprintf("%s/file%d-%d", dir, worker, j)

				data, err := writeFile(2000+mrand.Intn(5000), newFileName)
				if err != nil {
					t.Error(err)
					continue
				}
				filelock.Lock()
				files[newFileName] = data
				filelock.Unlock()
			}
		}(i)
	}

	wg.Wait()
	for name, data := range files {
		out, err := ioutil.ReadFile(name)
		if err != nil {
			t.Error(err)
		}

		if !bytes.Equal(data, out) {
			t.Errorf("Data didnt match in %s: expected %v, got %v", name, data, out)
		}
	}
}

// Test writing a medium sized file one byte at a time
func TestMultiWrite(t *testing.T) {

	if testing.Short() {
		t.SkipNow()
	}

	_, mnt := setupIpnsTest(t, nil)
	defer mnt.Close()

	fpath := mnt.Dir + "/local/file"
	fi, err := os.Create(fpath)
	if err != nil {
		t.Fatal(err)
	}

	data := randBytes(1001)
	for i := 0; i < len(data); i++ {
		n, err := fi.Write(data[i : i+1])
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatal("Somehow wrote the wrong number of bytes! (n != 1)")
		}
	}
	fi.Close()

	rbuf, err := ioutil.ReadFile(fpath)
	if err != nil {
		t.Fatal(err)
	}

	if !bytes.Equal(rbuf, data) {
		t.Fatal("File on disk did not match bytes written")
	}
}
