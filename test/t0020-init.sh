#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test init command"

. lib/test-lib.sh

test_expect_success "ipfs init succeeds" '
	export IPFS_DIR="$(pwd)/.go-ipfs" &&
	ipfs init
'

test_expect_success ".go-ipfs/ has been created" '
	test -d ".go-ipfs" &&
	test -f ".go-ipfs/config" &&
	test -d ".go-ipfs/datastore"
'

test_expect_success "ipfs config succeeds" '
	echo leveldb >expected &&
	ipfs config Datastore.Type >actual &&
	test_cmp expected actual
'

test_expect_success "ipfs -c='dir' init succeeds" '
	rm -r "$IPFS_DIR" &&
	unset IPFS_DIR &&
	ipfs -c="$(pwd)/.go-ipfs" init
'

test_expect_success "ipfs config Datastore.Path works" '
	echo "$(pwd)/.go-ipfs/datastore" >expected &&
	ipfs config Datastore.Path >actual &&
	test_cmp expected actual
'

test_expect_success "ipfs init fails when config exists" '
	test_must_fail ipfs -c="$(pwd)/.go-ipfs" init
'

test_expect_success "ipfs init -f=true succeeds when config exists" '
	ipfs -c="$(pwd)/.go-ipfs" init -f=true
'

test_done

