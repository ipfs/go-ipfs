
ifeq ($(TEST_NO_FUSE),1)
  go_test=go test -tags nofuse
else
  go_test=go test
endif

COMMIT := $(shell git rev-parse --short HEAD)
ldflags = "-X "github.com/ipfs/go-ipfs/repo/config".CurrentCommit=$(COMMIT)"
MAKEFLAGS += --no-print-directory

all: help

godep:
	go get github.com/tools/godep

gx:
	go get -u github.com/whyrusleeping/gx
	go get -u github.com/whyrusleeping/gx-go

deps: gx
	gx install --global

# saves/vendors third-party dependencies to Godeps/_workspace
# -r flag rewrites import paths to use the vendored path
# ./... performs operation on all packages in tree
vendor: godep
	godep save -r ./...

install: deps
	cd cmd/ipfs && go install -ldflags=$(ldflags)

build:
	cd cmd/ipfs && go build -i -ldflags=$(ldflags)

nofuse:
	cd cmd/ipfs && go install -tags nofuse -ldflags=$(ldflags)

clean:
	cd cmd/ipfs && go clean -ldflags=$(ldflags)

uninstall:
	cd cmd/ipfs && go clean -i -ldflags=$(ldflags)

PHONY += all help godep install build nofuse clean uninstall

##############################################################
# tests targets

test: test_expensive windows_build_check

test_short: build test_go_short test_sharness_short

test_expensive: build test_go_expensive test_sharness_expensive

test_3node:
	cd test/3nodetest && make

test_go_short:
	$(go_test) -test.short ./...

test_go_expensive:
	$(go_test) ./...

test_go_race:
	$(go_test) ./... -race

test_sharness_short:
	cd test/sharness/ && make

test_sharness_expensive:
	cd test/sharness/ && TEST_EXPENSIVE=1 make

test_all_commits:
	@echo "testing all commits between origin/master..HEAD"
	@echo "WARNING: this will 'git rebase --exec'."
	@test/bin/continueyn
	GIT_EDITOR=true git rebase -i --exec "make test" origin/master

test_all_commits_travis:
	# these are needed because travis.
	# we don't use this yet because it takes way too long.
	git config --global user.email "nemo@ipfs.io"
	git config --global user.name "IPFS BOT"
	git fetch origin master:master
	GIT_EDITOR=true git rebase -i --exec "make test" master

# since we have CI for osx and linux but not windows, this should help
windows_build_check:
	GOOS=windows GOARCH=amd64 go build -o .test.ipfs.exe ./cmd/ipfs
	rm .test.ipfs.exe

PHONY += test test_short test_expensive

##############################################################
# A semi-helpful help message

help:
	@echo 'DEPENDENCY TARGETS:'
	@echo ''
	@echo '  vendor       - Create a Godep workspace of 3rd party dependencies'
	@echo ''
	@echo 'BUILD TARGETS:'
	@echo ''
	@echo '  all          - print this help message'
	@echo '  build        - Build binary at ./cmd/ipfs/ipfs'
	@echo '  nofuse       - Build binary with no fuse support'
	@echo '  install      - Build binary and install into $$GOPATH/bin'
#	@echo '  dist_install - TODO: c.f. ./cmd/ipfs/dist/README.md'
	@echo ''
	@echo 'CLEANING TARGETS:'
	@echo ''
	@echo '  clean        - Remove binary from build directory'
	@echo '  uninstall    - Remove binary from $$GOPATH/bin'
	@echo ''
	@echo 'TESTING TARGETS:'
	@echo ''
	@echo '  test                    - Run expensive tests and Window$$ check'
	@echo '  test_short              - Run short tests and sharness tests'
	@echo '  test_expensive          - Run a few extras'
	@echo '  test_3node'
	@echo '  test_go_short'
	@echo '  test_go_expensive'
	@echo '  test_go_race'
	@echo '  test_sharness_short'
	@echo '  test_sharness_expensive'
	@echo '  test_all_commits'
	@echo "  test_all_commits_travis - DON'T USE: takes way too long"
	@echo '  windows_build_check'
	@echo ''

PHONY += help

.PHONY: $(PHONY)
