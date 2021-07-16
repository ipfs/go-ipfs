package plugin

import (
	"reflect"
	"testing"

	s3ds "github.com/ipfs/go-ds-s3"
)

func TestS3PluginDatastoreConfigParser(t *testing.T) {
	testcases := []struct {
		Input  map[string]interface{}
		Want   *S3Config
		HasErr bool
	}{
		{
			// Default case
			Input: map[string]interface{}{
				"region":    "someregion",
				"bucket":    "somebucket",
				"accessKey": "someaccesskey",
				"secretKey": "somesecretkey",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:    "someregion",
				Bucket:    "somebucket",
				AccessKey: "someaccesskey",
				SecretKey: "somesecretkey",
			}},
		},
		{
			// Required fields missing
			Input: map[string]interface{}{
				"region": "someregion",
			},
			HasErr: true,
		},
		{
			// Optional fields included
			Input: map[string]interface{}{
				"region":              "someregion",
				"bucket":              "somebucket",
				"accessKey":           "someaccesskey",
				"secretKey":           "somesecretkey",
				"sessionToken":        "somesessiontoken",
				"rootDirectory":       "/some/path",
				"regionEndpoint":      "someendpoint",
				"workers":             42.0,
				"credentialsEndpoint": "somecredendpoint",
			},
			Want: &S3Config{cfg: s3ds.Config{
				Region:              "someregion",
				Bucket:              "somebucket",
				AccessKey:           "someaccesskey",
				SecretKey:           "somesecretkey",
				SessionToken:        "somesessiontoken",
				RootDirectory:       "/some/path",
				RegionEndpoint:      "someendpoint",
				Workers:             42,
				CredentialsEndpoint: "somecredendpoint",
			}},
		},
	}

	for i, tc := range testcases {
		cfg, err := S3Plugin{}.DatastoreConfigParser()(tc.Input)
		if err != nil {
			if tc.HasErr {
				continue
			}
			t.Errorf("case %d: Failed to parse: %s", i, err)
			continue
		}
		if got, ok := cfg.(*S3Config); !ok {
			t.Errorf("wrong config type returned: %T", cfg)
		} else if !reflect.DeepEqual(got, tc.Want) {
			t.Errorf("case %d: got: %v; want %v", i, got, tc.Want)
		}
	}

}
