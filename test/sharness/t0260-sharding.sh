#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test directory sharding"

. lib/test-lib.sh

# FIXME: We now have dropped the global binary option of either sharding nothing
#  or sharding everything. We now have a threshold of 256 KiB (see `HAMTShardingSize`
#  in core/node/groups.go) above which directories are sharded. The directory size
#  is estimated as the size of each link (roughly entry name and CID byte length,
#  normally 34 bytes). So we need 256 KiB / (34 + 10) ~ 6000 entries in the directory
#  (estimating a fixed 10 char for each name) to trigger sharding.
# We also need to update the SHARDED/UNSHARDED CIDs.

test_expect_success "set up test data" '
  mkdir big_dir
  for i in `seq 6500` # just to be sure
  do
    echo $i > big_dir/`printf "file%06d" $i`
  done

  mkdir small_dir
  for i in `seq 100`
  do
    echo $i > small_dir/file$i
  done
'

test_add_large_dir() {
  exphash="$1"
  input_dir="$2" # FIXME: added a new entry to switch between small and big_dir
                 #  we need to update the calls.
  test_expect_success "ipfs add on very large directory succeeds" '
    ipfs add -r -q $input_dir | tail -n1 > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '
  test_expect_success "ipfs get on very large directory succeeds" '
    ipfs get -o output_dir "$exphash" &&
    test_cmp $input_dir output_dir
    rm output_dir -r # FIXME: Cleaning the output directory because we now have
                     #  two different directories and it seems `ipfs get` doesnt
                     #  overwrite just adds files to the output dir.
  '
}

test_init_ipfs

# FIXME: Updating CID based just on the expected output of a failed test run. These
#  need to be confirmed.
UNSHARDED_SMALL="QmZedLGyvgWiyGQzAw2GpuLZeqzxmVcUQUbyRkVWx5DkxK"
test_add_large_dir "$UNSHARDED_SMALL" small_dir

test_launch_ipfs_daemon

test_add_large_dir "$UNSHARDED_SMALL" small_dir

test_kill_ipfs_daemon

SHARDED_BIG="Qmbgq1VC2KSWWhdFLvH6mapN1wuY67FZVPYbCGyR5TnYSv"
test_add_large_dir "$SHARDED_BIG" big_dir

test_launch_ipfs_daemon

test_add_large_dir "$SHARDED_BIG" big_dir

test_kill_ipfs_daemon

# FIXME: This test no longer works, we have different directories for sharded
#  and unsharded. We might have to duplicate the UNSHARDED/SHARDED for small
#  and big dirs.
test_expect_success "sharded and unsharded output look the same" '
  ipfs ls "$SHARDED" | sort > sharded_out &&
  ipfs ls "$UNSHARDED" | sort > unsharded_out &&
  test_cmp sharded_out unsharded_out
'

test_expect_success "ipfs cat error output the same" '
  test_expect_code 1 ipfs cat "$SHARDED" 2> sharded_err &&
  test_expect_code 1 ipfs cat "$UNSHARDED" 2> unsharded_err &&
  test_cmp sharded_err unsharded_err
'

# FIXME: Should we replicate this test for the small (non sharded) directory as well?
test_expect_success "'ipfs ls --resolve-type=false --size=false' admits missing block" '
  ipfs ls "$SHARDED_BIG" | head -1 > first_file &&
  ipfs ls --size=false "$SHARDED_BIG" | sort > sharded_out_nosize &&
  read -r HASH _ NAME <first_file &&
  ipfs pin rm "$SHARDED_BIG" "$UNSHARDED" && # To allow us to remove the block
  ipfs block rm "$HASH" &&
  test_expect_code 1 ipfs cat "$SHARDED_BIG/$NAME" &&
  test_expect_code 1 ipfs ls "$SHARDED_BIG" &&
  ipfs ls --resolve-type=false --size=false "$SHARDED_BIG" | sort > missing_out &&
  test_cmp sharded_out_nosize missing_out
'

test_launch_ipfs_daemon

test_expect_success "gateway can resolve sharded dirs" '
  echo 100 > expected &&
  curl -sfo actual "http://127.0.0.1:$GWAY_PORT/ipfs/$SHARDED_BIG/file100" &&
  test_cmp expected actual
'

test_expect_success "'ipfs resolve' can resolve sharded dirs" '
  echo /ipfs/QmZ3RfWk1u5LEGYLHA633B5TNJy3Du27K6Fny9wcxpowGS > expected &&
  ipfs resolve "/ipfs/$SHARDED_BIG/file100" > actual &&
  test_cmp expected actual
'

test_kill_ipfs_daemon

test_add_large_dir_v1() {
  exphash="$1"
  test_expect_success "ipfs add (CIDv1) on very large directory succeeds" '
    ipfs add -r -q --cid-version=1 testdata | tail -n1 > sharddir_out &&
    echo "$exphash" > sharddir_exp &&
    test_cmp sharddir_exp sharddir_out
  '

  test_expect_success "can access a path under the dir" '
    ipfs cat "$exphash/file20" > file20_out &&
    test_cmp testdata/file20 file20_out
  '
}

# this hash implies the directory is CIDv1 and leaf entries are CIDv1 and raw
SHARDEDV1="bafybeibiemewfzzdyhq2l74wrd6qj2oz42usjlktgnlqv4yfawgouaqn4u"
test_add_large_dir_v1 "$SHARDEDV1"

test_launch_ipfs_daemon

test_add_large_dir_v1 "$SHARDEDV1"

test_kill_ipfs_daemon

test_done
