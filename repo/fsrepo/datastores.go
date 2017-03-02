package fsrepo

import (
	"fmt"
	"path/filepath"

	repo "github.com/ipfs/go-ipfs/repo"

	s3ds "github.com/ipfs/go-ds-s3"
	ds "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore"
	mount "gx/ipfs/QmRWDav6mzWseLWeYfVd5fvUKiVe9xNH29YfMF438fG364/go-datastore/syncmount"
	levelds "gx/ipfs/QmaHHmfEozrrotyhyN44omJouyuEtx6ahddqV6W5yRaUSQ/go-ds-leveldb"
	ldbopts "gx/ipfs/QmbBhyDKsY4mbY6xsKt3qu9Y7FPvMJ6qbD8AMjYYvPRw1g/goleveldb/leveldb/opt"
	"gx/ipfs/QmbUSMTQtK9GRrUbD4ngqJwSzHsquUc8nyDubRWp4vPybH/go-ds-measure"
	"gx/ipfs/Qmbx2KUs8mUbDUiiESzC1ms7mdmh4pRu8X1V1tffC46M4n/go-ds-flatfs"
)

func (r *FSRepo) constructDatastore(params map[string]interface{}) (repo.Datastore, error) {
	switch params["type"] {
	case "mount":
		mounts, ok := params["mounts"].([]interface{})
		if !ok {
			return nil, fmt.Errorf("mounts field wasnt an array")
		}

		return r.openMountDatastore(mounts)
	case "flatfs":
		return r.openFlatfsDatastore(params)
	case "mem":
		return ds.NewMapDatastore(), nil
	case "log":
		child, err := r.constructDatastore(params["child"].(map[string]interface{}))
		if err != nil {
			return nil, err
		}

		return ds.NewLogDatastore(child, params["name"].(string)), nil
	case "measure":
		child, err := r.constructDatastore(params["child"].(map[string]interface{}))
		if err != nil {
			return nil, err
		}

		prefix := params["prefix"].(string)

		return r.openMeasureDB(prefix, child)

	case "s3ds":
		return r.openS3Datastore(params)
	case "levelds":
		return r.openLeveldbDatastore(params)
	default:
		return nil, fmt.Errorf("unknown datastore type: %s", params["type"])
	}
}

func (r *FSRepo) openMountDatastore(mountcfg []interface{}) (repo.Datastore, error) {
	var mounts []mount.Mount
	for _, iface := range mountcfg {
		cfg := iface.(map[string]interface{})

		child, err := r.constructDatastore(cfg)
		if err != nil {
			return nil, err
		}

		prefix, found := cfg["mountpoint"]
		if !found {
			return nil, fmt.Errorf("no 'mountpoint' on mount")
		}

		mounts = append(mounts, mount.Mount{
			Datastore: child,
			Prefix:    ds.NewKey(prefix.(string)),
		})
	}

	return mount.New(mounts), nil
}

func (r *FSRepo) openFlatfsDatastore(params map[string]interface{}) (repo.Datastore, error) {
	p := params["path"].(string)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}

	plen := int(params["prefixLen"].(float64))
	return flatfs.New(p, plen, params["nosync"].(bool))
}

func (r *FSRepo) openLeveldbDatastore(params map[string]interface{}) (repo.Datastore, error) {
	p := params["path"].(string)
	if !filepath.IsAbs(p) {
		p = filepath.Join(r.path, p)
	}

	var c ldbopts.Compression
	switch params["compression"].(string) {
	case "none":
		c = ldbopts.NoCompression
	case "snappy":
		c = ldbopts.SnappyCompression
	case "":
		fallthrough
	default:
		c = ldbopts.DefaultCompression
	}
	return levelds.NewDatastore(p, &levelds.Options{
		Compression: c,
	})
}

func (r *FSRepo) openMeasureDB(prefix string, child repo.Datastore) (repo.Datastore, error) {
	return measure.New(prefix, child), nil
}

func (r *FSRepo) openS3Datastore(params map[string]interface{}) (repo.Datastore, error) {
	keys := []string{
		"bucket",
		"domain",
		"secretKey",
		"accessKey",
	}
	if err := assertStrings("s3ds", params, keys); err != nil {
		return nil, err
	}
	cfg := &s3ds.Config{
		Bucket:    params["bucket"].(string),
		Domain:    params["domain"].(string),
		SecretKey: params["secretKey"].(string),
		AccessKey: params["accessKey"].(string),
	}

	return s3ds.NewS3Datastore(cfg), nil
}

func assertStrings(dstype string, m map[string]interface{}, keys []string) error {
	for _, k := range keys {
		val, ok := m[k]
		if !ok {
			return fmt.Errorf("%s config: key '%s' not found", dstype, k)
		}
		_, ok = val.(string)
		if !ok {
			return fmt.Errorf("%s config: key '%s' was not a string", dstype, k)
		}
	}
	return nil
}
