package plugin

import (
	"fmt"

	s3ds "github.com/ipfs/go-ds-s3"
	"github.com/ipfs/go-ipfs/plugin"
	"github.com/ipfs/go-ipfs/repo"
	"github.com/ipfs/go-ipfs/repo/fsrepo"
)

var Plugins = []plugin.Plugin{
	&S3Plugin{},
}

type S3Plugin struct{}

func (s3p S3Plugin) Name() string {
	return "s3-datastore-plugin"
}

func (s3p S3Plugin) Version() string {
	return "0.0.1"
}

func (s3p S3Plugin) Init(env *plugin.Environment) error {
	return nil
}

func (s3p S3Plugin) DatastoreTypeName() string {
	return "s3ds"
}

func (s3p S3Plugin) DatastoreConfigParser() fsrepo.ConfigFromMap {
	return func(m map[string]interface{}) (fsrepo.DatastoreConfig, error) {
		region, ok := m["region"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no region specified")
		}

		bucket, ok := m["bucket"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no bucket specified")
		}

		accessKey, ok := m["accessKey"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no accessKey specified")
		}

		secretKey, ok := m["secretKey"].(string)
		if !ok {
			return nil, fmt.Errorf("s3ds: no secretKey specified")
		}

		// Optional.

		var sessionToken string
		if v, ok := m["sessionToken"]; ok {
			sessionToken, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: sessionToken not a string")
			}
		}

		var endpoint string
		if v, ok := m["regionEndpoint"]; ok {
			endpoint, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: regionEndpoint not a string")
			}
		}
		var rootDirectory string
		if v, ok := m["rootDirectory"]; ok {
			rootDirectory, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: rootDirectory not a string")
			}
		}
		var workers int
		if v, ok := m["workers"]; ok {
			workersf, ok := v.(float64)
			workers = int(workersf)
			switch {
			case !ok:
				return nil, fmt.Errorf("s3ds: workers not a number")
			case workers <= 0:
				return nil, fmt.Errorf("s3ds: workers <= 0: %f", workersf)
			case float64(workers) != workersf:
				return nil, fmt.Errorf("s3ds: workers is not an integer: %f", workersf)
			}
		}
		var credentialsEndpoint string
		if v, ok := m["credentialsEndpoint"]; ok {
			credentialsEndpoint, ok = v.(string)
			if !ok {
				return nil, fmt.Errorf("s3ds: credentialsEndpoint not a string")
			}
		}

		return &S3Config{
			cfg: s3ds.Config{
				Region:              region,
				Bucket:              bucket,
				AccessKey:           accessKey,
				SecretKey:           secretKey,
				SessionToken:        sessionToken,
				RootDirectory:       rootDirectory,
				Workers:             workers,
				RegionEndpoint:      endpoint,
				CredentialsEndpoint: credentialsEndpoint,
			},
		}, nil
	}
}

type S3Config struct {
	cfg s3ds.Config
}

func (s3c *S3Config) DiskSpec() fsrepo.DiskSpec {
	return fsrepo.DiskSpec{
		"region":        s3c.cfg.Region,
		"bucket":        s3c.cfg.Bucket,
		"rootDirectory": s3c.cfg.RootDirectory,
	}
}

func (s3c *S3Config) Create(path string) (repo.Datastore, error) {
	return s3ds.NewS3Datastore(s3c.cfg)
}
