#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_init_ipfs

# test publishing a hash


test_expect_success "'ipfs name publish --allow-offline' succeeds" '
  PEERID=`ipfs id --format="<id>"` &&
  test_check_peerid "${PEERID}" &&
  ipfs name publish --allow-offline  "/ipfs/$HASH_WELCOME_DOCS" >publish_out
'

test_expect_success "publish output looks good" '
  echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS" >expected1 &&
  test_cmp expected1 publish_out
'

test_expect_success "'ipfs name resolve' succeeds" '
  ipfs name resolve "$PEERID" >output
'

test_expect_success "resolve output looks good" '
  printf "/ipfs/%s\n" "$HASH_WELCOME_DOCS" >expected2 &&
  test_cmp expected2 output
'

# test resolve key

test_expect_success "'ipfs name resolve self' succeeds" '
  ipfs name resolve self >self_output
'

test_expect_success "self resolve output looks good" '
  test_cmp expected2 self_output
'

test_expect_success "'ipfs name publish --key=test_key' succeeds" '
  TEST_KEY_HASH=$(ipfs key gen test_key --type=rsa --size=2048)
  ipfs name publish --key=test_key "/ipfs/$HASH_WELCOME_DOCS" >publish_out
'

test_expect_success "test_key publish output looks good" '
  echo "Published to ${TEST_KEY_HASH}: /ipfs/$HASH_WELCOME_DOCS" >expected1 &&
  test_cmp expected1 publish_out
'

test_expect_success "'ipfs name resolve test_key' succeeds" '
  ipfs name resolve test_key > test_key_output
'

test_expect_success "test_key resolve output looks good" '
  test_cmp expected2 test_key_output
  ipfs key rm test_key
'

# now test with a path

test_expect_success "'ipfs name publish --allow-offline' succeeds" '
  PEERID=`ipfs id --format="<id>"` &&
  test_check_peerid "${PEERID}" &&
  ipfs name publish --allow-offline "/ipfs/$HASH_WELCOME_DOCS/help" >publish_out
'

test_expect_success "publish a path looks good" '
  echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS/help" >expected3 &&
  test_cmp expected3 publish_out
'

test_expect_success "'ipfs name resolve' succeeds" '
  ipfs name resolve "$PEERID" >output
'

test_expect_success "resolve output looks good" '
  printf "/ipfs/%s/help\n" "$HASH_WELCOME_DOCS" >expected4 &&
  test_cmp expected4 output
'

test_expect_success "ipfs cat on published content succeeds" '
  ipfs cat "/ipfs/$HASH_WELCOME_DOCS/help" >expected &&
  ipfs cat "/ipns/$PEERID" >actual &&
  test_cmp expected actual
'

# publish with an explicit node ID

test_expect_failure "'ipfs name publish --allow-offline <local-id> <hash>' succeeds" '
  PEERID=`ipfs id --format="<id>"` &&
  test_check_peerid "${PEERID}" &&
  echo ipfs name publish --allow-offline "${PEERID}" "/ipfs/$HASH_WELCOME_DOCS" &&
  ipfs name publish --allow-offline "${PEERID}" "/ipfs/$HASH_WELCOME_DOCS" >actual_node_id_publish
'

test_expect_failure "publish with our explicit node ID looks good" '
  echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS" >expected_node_id_publish &&
  test_cmp expected_node_id_publish actual_node_id_publish
'

# publish with an explicit node ID as key name

test_expect_success "generate and verify a new key" '
  NEWID=`ipfs key gen --type=rsa --size=2048 keyname` &&
  test_check_peerid "${NEWID}"
'

test_expect_success "'ipfs name publis --allow-offline --key=<peer-id> <hash>' succeeds" '
  ipfs name publish --allow-offline  --key=${NEWID} "/ipfs/$HASH_WELCOME_DOCS" >actual_node_id_publish
'

test_expect_success "publish an explicit node ID as key name looks good" '
  echo "Published to ${NEWID}: /ipfs/$HASH_WELCOME_DOCS" >expected_node_id_publish &&
  test_cmp expected_node_id_publish actual_node_id_publish
'


# test publishing nothing

test_expect_success "'ipfs name publish' fails" '
  printf '' | test_expect_code 1 ipfs name publish --allow-offline  >publish_out 2>&1
'

test_expect_success "publish output has the correct error" '
  grep "argument \"ipfs-path\" is required" publish_out
'

test_expect_success "'ipfs name publish --help' succeeds" '
  ipfs name publish --help
'

test_launch_ipfs_daemon

test_expect_success "empty request to name publish doesn't panic and returns error" '
  curl "http://$API_ADDR/api/v0/name/publish" > curl_out || true &&
    grep "argument \"ipfs-path\" is required" curl_out
'

test_kill_ipfs_daemon


# Test daemon in offline mode
test_launch_ipfs_daemon --offline

test_expect_success "'ipfs name publish' fails offline mode" '
  PEERID=`ipfs id --format="<id>"` &&
  test_check_peerid "${PEERID}" &&
  test_expect_code 1 ipfs name publish "/ipfs/$HASH_WELCOME_DOCS"
'

test_kill_ipfs_daemon

test_done
