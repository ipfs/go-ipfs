#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="test bitswap commands"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

test_expect_success "'ipfs block get' adds hash to wantlist" '
	export NONEXIST=QmeXxaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa &&
	test_expect_code 1 ipfs block get $NONEXIST --timeout=10ms &&
	ipfs bitswap wantlist >wantlist_out &&
	grep $NONEXIST wantlist_out
'

test_expect_success "'ipfs bitswap unwant' succeeds" '
	ipfs bitswap unwant $NONEXIST
'

test_expect_success "hash was removed from wantlist" '
	ipfs bitswap wantlist > wantlist_out &&
	test_must_be_empty wantlist_out
'

test_kill_ipfs_daemon

test_done
