#!/usr/bin/env bash
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="CID Version 0/1 Duality"

. lib/test-lib.sh

test_init_ipfs

#
#
#

test_expect_success "create two small files" '
  random 1000 7 > afile
  random 1000 9 > bfile
'

test_expect_success "add file using CIDv1 but don't pin" '
  AHASHv1=$(ipfs add -q --cid-version=1 --raw-leaves=false --pin=false afile)
'

test_expect_success "add file using CIDv0" '
  AHASHv0=$(ipfs add -q --cid-version=0 afile)
'

test_expect_success "check hashes" '
  test "$(cid-fmt %v-%c $AHASHv0)" = "cidv0-protobuf" &&
  test "$(cid-fmt %v-%c $AHASHv1)" = "cidv1-protobuf" &&
  test "$(cid-fmt -v 0 %s $AHASHv1)" = "$AHASHv0"
'

test_expect_success "make sure CIDv1 hash really is in the repo" '
  ipfs refs local | grep -q $(cid-fmt -b f %M $AHASHv1)
'

test_expect_success "make sure CIDv0 hash really is in the repo" '
  ipfs refs local | grep -q $(cid-fmt -b f %M $AHASHv0)
'

test_expect_success "run gc" '
  ipfs repo gc
'

test_expect_success "make sure the CIDv0 hash is in the repo" '
  ipfs refs local | grep -q $(cid-fmt -b f %M $AHASHv0)
'

test_expect_success "make sure we can get CIDv0 added file" '
  ipfs cat $AHASHv0 > thefile &&
  test_cmp afile thefile
'

test_expect_success "clean up" '
  ipfs pin rm $AHASHv0 &&
  ipfs repo gc &&
  ! ipfs refs local | grep -q $(cid-fmt -b f %M $AHASHv0)
'

#
#
#

test_expect_success "add file using CIDv1 but don't pin" '
  ipfs add -q --cid-version=1 --raw-leaves=false --pin=false afile
'

test_expect_success "check that we can access the file when converted to CIDv0" '
  ipfs cat $AHASHv0 > thefile &&
  test_cmp afile thefile
'

test_expect_success "clean up" '
  ipfs repo gc
'

test_expect_success "add file using CIDv0 but don't pin" '
  ipfs add -q --cid-version=0 --raw-leaves=false --pin=false afile
'

test_expect_success "check that we can access the file when converted to CIDv1" '
  ipfs cat $AHASHv1 > thefile &&
  test_cmp afile thefile
'

#
#
#

test_expect_success "set up iptb testbed" '
  iptb init -n 2 -p 0 -f --bootstrap=none
'

test_expect_success "start nodes" '
  iptb start &&
  iptb connect 0 1
'

test_expect_success "add afile using CIDv0 to node 0" '
  iptb run 0 ipfs add -q --cid-version=0 afile
'

test_expect_success "get afile using CIDv1 via node 1" '
  iptb run 1 ipfs --timeout=2s cat $AHASHv1 > thefile &&
  test_cmp afile thefile
'

test_expect_success "add bfile using CIDv1 to node 0" '
  BHASHv1=$(iptb run 0 ipfs add -q --cid-version=1 --raw-leaves=false bfile)
'

test_expect_success "get bfile using CIDv0 via node 1" '
  BHASHv0=$(cid-fmt -v 0 %s $BHASHv1)
  iptb run 1 ipfs --timeout=2s cat $BHASHv0 > thefile &&
  test_cmp bfile thefile
'

test_expect_success "stop testbed" '
  iptb stop
'

test_done
