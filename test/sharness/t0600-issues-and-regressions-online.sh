#!/usr/bin/env bash

test_description="Tests for various fixed issues and regressions."

. lib/test-lib.sh

test_init_ipfs

test_launch_ipfs_daemon

# Tests go here

test_expect_success "commands command with flag flags works via HTTP API - #2301" '
  curl -X POST "http://$API_ADDR/api/v0/commands?flags" | grep "verbose"
'

test_expect_success "ipfs refs local over HTTP API returns NDJOSN not flat - #2803" '
  echo "Hello World" | ipfs add &&
  curl -X POST "http://$API_ADDR/api/v0/refs/local" | grep "Ref" | grep "Err"
'

test_expect_success "args expecting stdin don't crash when not given" '
  curl -X POST "$API_ADDR/api/v0/bootstrap/add" > result
'

test_expect_success "no panic traces on daemon" '
  test_must_fail grep "nil pointer dereference" daemon_err
'

test_expect_success "metrics work" '
  curl -X POST "$API_ADDR/debug/metrics/prometheus" > pro_data &&
  grep "ipfs_bs_cache_arc_hits_total" < pro_data ||
  test_fsh cat pro_data
'

test_expect_success "pin add api looks right - #3753" '
  HASH=$(echo "foo" | ipfs add -q) &&
  curl -X POST "http://$API_ADDR/api/v0/pin/add/$HASH" > pinadd_out &&
  echo "{\"Pins\":[\"QmYNmQKp6SuaVrpgWRsPTgCQCnpxUYGq76YEKBXuj2N4H6\"]}" > pinadd_exp &&
  test_cmp pinadd_out pinadd_exp
'

test_expect_success "pin add api looks right - #3753" '
  curl -X POST "http://$API_ADDR/api/v0/pin/rm/$HASH" > pinrm_out &&
  echo "{\"Pins\":[\"QmYNmQKp6SuaVrpgWRsPTgCQCnpxUYGq76YEKBXuj2N4H6\"]}" > pinrm_exp &&
  test_cmp pinrm_out pinrm_exp
'

test_expect_success SOCAT "no daemon crash on improper file argument - #4003 ( test needs socat )" '
  FNC=$(echo $API_ADDR | awk -F: '\''{ printf "%s:%s", $1, $2 }'\'') &&
  printf "POST /api/v0/add?pin=true HTTP/1.1\r\nHost: $API_ADDR\r\nContent-Type: multipart/form-data; boundary=Pyw9xQLtiLPE6XcI\r\nContent-Length: 22\r\n\r\n\r\n--Pyw9xQLtiLPE6XcI\r\n" | socat STDIO tcp-connect:$FNC | grep -m1 "500 Internal Server Error"
'

test_kill_ipfs_daemon

test_expect_success "ipfs daemon --offline --mount fails - #2995" '
  test_expect_code 1 ipfs daemon --offline --mount 2>daemon_err &&
  grep "mount is not currently supported in offline mode" daemon_err ||
  test_fsh cat daemon_err
'

test_launch_ipfs_daemon_without_network

test_expect_success "'ipfs name resolve' succeeds after ipfs id when daemon offline" '
  PEERID=`ipfs key list --ipns-base=base36 -l | grep self | cut -d " " -f1` &&
  test_check_peerid "${PEERID}" &&
  ipfs name publish --allow-offline  -Q "/ipfs/$HASH_WELCOME_DOCS" >publish_out
'

test_expect_success "pubrmlish --quieter output looks good" '
  echo "${PEERID}" >expected1 &&
  test_cmp expected1 publish_out
'

test_expect_success "'ipfs name resolve' succeeds" '
  ipfs name resolve "$PEERID" >output
'

test_expect_success "resolve output looks good" '
  printf "/ipfs/%s\n" "$HASH_WELCOME_DOCS" >expected2 &&
  test_cmp expected2 output
'

test_kill_ipfs_daemon

test_done

