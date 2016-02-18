FROM alpine:3.3
MAINTAINER Lars Gierth <lgierth@ipfs.io>

# There is a copy of this Dockerfile in test/sharness,
# which is optimized for build time, instead of image size.
#
# Please keep these two Dockerfiles in sync.


# Ports for Swarm TCP, Swarm uTP, API, Gateway
EXPOSE 4001
EXPOSE 4002/udp
EXPOSE 5001
EXPOSE 8080

# Volume for mounting an IPFS fs-repo
# This is moved to the bottom for technical reasons.
#VOLUME $IPFS_PATH

# IPFS API to use for fetching gx packages.
# This can be a gateway too, since its read-only API provides all gx needs.
# - e.g. /ip4/172.17.0.1/tcp/8080 if the Docker host
#   has the IPFS gateway listening on the bridge interface
#   provided by Docker's default networking.
# - if empty, the public gateway at ipfs.io is used.
ENV GX_IPFS   ""
# The IPFS fs-repo within the container
ENV IPFS_PATH /data/ipfs
# Golang stuff
ENV GO_VERSION 1.5.3-r0
ENV GOPATH     /go
ENV PATH       /go/bin:$PATH
ENV SRC_PATH   /go/src/github.com/ipfs/go-ipfs

# Get the go-ipfs sourcecode
COPY . $SRC_PATH

RUN apk add --update musl go=$GO_VERSION git bash wget ca-certificates \
	# Setup user and fs-repo directory
	&& mkdir -p $IPFS_PATH \
	&& adduser -D -h $IPFS_PATH -u 1000 ipfs \
	&& chown ipfs:ipfs $IPFS_PATH && chmod 755 $IPFS_PATH \
	# Install gx
	&& go get -u github.com/whyrusleeping/gx \
	&& go get -u github.com/whyrusleeping/gx-go \
	# Point gx to a specific IPFS API
	&& ([ -z "$GX_IPFS" ] || echo $GX_IPFS > $IPFS_PATH/api) \
	# Invoke gx
	&& cd $SRC_PATH \
	&& gx --verbose install --global \
	# We get the current commit using this hack,
	# so that we don't have to copy all of .git/ into the build context.
	# This saves us quite a bit of image size.
	&& ref="$(cat .git/HEAD | cut -d' ' -f2)" \
	&& commit="$(cat .git/$ref | head -c 7)" \
	&& echo "ldflags=-X github.com/ipfs/go-ipfs/repo/config.CurrentCommit=$commit" \
	# Build and install IPFS and entrypoint script
	&& cd $SRC_PATH/cmd/ipfs \
	&& go build -ldflags "-X github.com/ipfs/go-ipfs/repo/config.CurrentCommit=$commit" \
	&& cp ipfs /usr/local/bin/ipfs \
	&& cp $SRC_PATH/bin/container_daemon /usr/local/bin/start_ipfs \
	&& chmod 755 /usr/local/bin/start_ipfs \
	# Remove all build-time dependencies
	&& apk del --purge musl go git && rm -rf $GOPATH && rm -vf $IPFS_PATH/api

# Call uid 1000 "ipfs"
USER ipfs

# Expose the fs-repo as a volume.
# We're doing this down here (and not at the top),
# so that the overlay directory is owned by the ipfs user.
# start_ipfs initializes an ephemeral fs-repo if none is mounted,
# which is why uid=1000 needs write permissions there.
VOLUME $IPFS_PATH

# This just makes sure that:
# 1. There's an fs-repo, and initializes one if there isn't.
# 2. The API and Gateway are accessible from outside the container.
ENTRYPOINT ["/usr/local/bin/start_ipfs"]
