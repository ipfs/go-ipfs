#!/usr/bin/env bash
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Cid Security"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "adding using unsafe function fails with error" '
  echo foo | test_must_fail ipfs add --hash murmur3 2>add_out
'

test_expect_success "error reason is pointed out" '
  grep "insecure hash functions not allowed" add_out
'

test_expect_success "adding using too short of a hash function gives out an error" '
  echo foo | test_must_fail ipfs block put -f protobuf --mhlen 19 2>block_out
'

test_expect_success "error reason is pointed out" '
  grep "hashes must be at 20 least bytes long" block_out
'


test_cat_get() {

  test_expect_success "ipfs cat fails with unsafe hash function" '
    test_must_fail ipfs cat zDvnoLcPKWR 2>ipfs_cat
  '


  test_expect_success "error reason is pointed out" '
    grep "insecure hash functions not allowed" ipfs_cat
  '


  test_expect_success "ipfs get fails with too short function" '
    test_must_fail ipfs get z2ba5YhCCFNFxLtxMygQwjBjYSD8nUeN 2>ipfs_get

    '

  test_expect_success "error reason is pointed out" '
     grep "hashes must be at 20 least bytes long" ipfs_get
  '
}


test_gc() {
  test_expect_success "injecting insecure block" '
    mkdir -p "$IPFS_PATH/blocks/TS" &&
    cp -f ../t0275-cid-security-data/AFKSEBCGPUJZE.data "$IPFS_PATH/blocks/TS/EICEM7ITSI.data"
  '

  test_expect_success "gc works" 'ipfs repo gc > gc_out'
  test_expect_success "gc removed bad block" '
    grep $(cid-fmt -b f %M zDvnoGUyhEq) gc_out
  '
}


# should work offline
test_cat_get
test_gc

# should work online
test_launch_ipfs_daemon
test_cat_get
test_gc

test_expect_success "add block linking to insecure" '
  mkdir -p "$IPFS_PATH/blocks/5X" &&
  cp -f "../t0275-cid-security-data/CIQG6PGTD2VV34S33BE4MNCQITBRFYUPYQLDXYARR3DQW37MOT7K5XI.data" "$IPFS_PATH/blocks/5X"
'

test_expect_success "ipfs cat fails with code 1 and not timeout" '
  test_expect_code 1 go-timeout 1s ipfs cat QmVpsktzNeJdfWEpyeix93QJdQaBSgRNxebSbYSo9SQPGx
'

test_kill_ipfs_daemon

test_done
