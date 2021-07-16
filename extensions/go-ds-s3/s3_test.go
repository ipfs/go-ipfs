package s3ds

import (
	"os"
	"testing"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/s3"
	dstest "github.com/ipfs/go-datastore/test"
)

func TestSuite(t *testing.T) {
	// run docker-compose up in this repo in order to get a local
	// s3 running on port 4572
	config := Config{}
	_, hasLocalS3 := os.LookupEnv("LOCAL_S3")
	if hasLocalS3 {
		config = Config{
			RegionEndpoint: "http://localhost:4566",
			Bucket:         "localbucketname",
			Region:         "us-east-1",
			AccessKey:      "localonlyac",
			SecretKey:      "localonlysk",
		}
	}

	s3ds, err := NewS3Datastore(config)
	if err != nil {
		t.Fatal(err)
	}

	if hasLocalS3 {
		err = devMakeBucket(s3ds.S3, "localbucketname")
		if err != nil {
			t.Fatal(err)
		}
	}

	t.Run("basic operations", func(t *testing.T) {
		dstest.SubtestBasicPutGet(t, s3ds)
	})
	t.Run("not found operations", func(t *testing.T) {
		dstest.SubtestNotFounds(t, s3ds)
	})
	t.Run("many puts and gets, query", func(t *testing.T) {
		dstest.SubtestManyKeysAndQuery(t, s3ds)
	})
	t.Run("return sizes", func(t *testing.T) {
		dstest.SubtestReturnSizes(t, s3ds)
	})
}

func devMakeBucket(s3obj *s3.S3, bucketName string) error {
	_, err := s3obj.CreateBucket(&s3.CreateBucketInput{
		Bucket: aws.String(bucketName),
	})

	return err
}
