package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"time"

	aws "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/aws"
	s3 "github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/crowdmob/goamz/s3"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/fzzy/radix/redis"
	"github.com/ipfs/go-ipfs/Godeps/_workspace/src/github.com/jbenet/go-datastore"
	context "github.com/ipfs/go-ipfs/Godeps/_workspace/src/golang.org/x/net/context"
	core "github.com/ipfs/go-ipfs/core"
	corerouting "github.com/ipfs/go-ipfs/core/corerouting"
	config "github.com/ipfs/go-ipfs/repo/config"
	fsrepo "github.com/ipfs/go-ipfs/repo/fsrepo"
	redisds "github.com/ipfs/go-ipfs/thirdparty/redis-datastore"
	s3datastore "github.com/ipfs/go-ipfs/thirdparty/s3-datastore"
	ds2 "github.com/ipfs/go-ipfs/util/datastore2"
)

var (
	ttl             = flag.Duration("ttl", 12*time.Hour, "period after which routing keys expire")
	redisHost       = flag.String("redis-host", "localhost:6379", "redis tcp host address:port")
	redisPassword   = flag.String("redis-pass", "", "redis password if required")
	datastoreOption = flag.String("datastore", "redis", "routing datastore (also available: aws)")
	s3bucket        = flag.String("aws-bucket", "", "S3 bucket for aws routing datastore")
	s3region        = flag.String("aws-region", aws.USWest2.Name, "S3 region")
	nBitsForKeypair = flag.Int("b", 1024, "number of bits for keypair (if repo is uninitialized)")
)

func main() {
	flag.Parse()
	if err := run(); err != nil {
		log.Println(err)
	}
}

func run() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	repoPath, err := fsrepo.BestKnownPath()
	if err != nil {
		return err
	}

	if !fsrepo.IsInitialized(repoPath) {
		conf, err := config.Init(os.Stdout, *nBitsForKeypair)
		if err != nil {
			return err
		}
		if err := fsrepo.Init(repoPath, conf); err != nil {
			return err
		}
	}
	repo, err := fsrepo.Open(repoPath)
	if err != nil { // owned by node
		return err
	}

	var ds datastore.ThreadSafeDatastore
	switch *datastoreOption {
	case "redis":
		redisClient, err := redis.Dial("tcp", *redisHost)
		if err != nil {
			return fmt.Errorf("could not connect to redis: %s", err)
		}
		if *redisPassword != "" {
			if err := redisClient.Cmd("AUTH", *redisPassword).Err; err != nil {
				return err
			}
		}
		redisds, err := redisds.NewExpiringDatastore(redisClient, *ttl)
		if err != nil {
			return err
		}
		ds = redisds
	case "aws":
		s3raw, err := makeS3Datastore()
		if err != nil {
			return err
		}
		s3, err := enhanceDatastore(s3raw)
		if err != nil {
			return err
		}
		ds = s3
	default:
		return errors.New("unsupported datastore type")
	}
	node, err := core.NewIPFSNode(ctx,
		core.OnlineWithOptions(
			repo,
			corerouting.SupernodeServer(ds),
			core.DefaultHostOption),
	)
	if err != nil {
		return err
	}
	defer node.Close()

	interrupt := make(chan os.Signal)
	signal.Notify(interrupt, os.Kill, os.Interrupt)
	<-interrupt
	return nil
}

func makeS3Datastore() (*s3datastore.S3Datastore, error) {

	// FIXME get ENV through flags?

	auth, err := aws.EnvAuth()
	if err != nil {
		return nil, err
	}

	s3c := s3.New(auth, aws.Regions[*s3region])
	b := s3c.Bucket(*s3bucket)
	exists, err := b.Exists("initialized") // TODO lazily instantiate
	if err != nil {
		return nil, err
	}

	if !exists {
		if err := b.PutBucket(s3.PublicRead); err != nil {
			switch e := err.(type) {
			case *s3.Error:
				log.Println(e.Code)
			default:
				return nil, err
			}
		}

		// TODO create the initial value
	}

	return &s3datastore.S3Datastore{
		Bucket: *s3bucket,
		Client: s3c,
	}, nil
}

func enhanceDatastore(d datastore.ThreadSafeDatastore) (datastore.ThreadSafeDatastore, error) {
	// TODO cache
	return ds2.CloserWrap(d), nil
}
