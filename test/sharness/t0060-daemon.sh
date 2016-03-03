#!/bin/sh
#
# Copyright (c) 2014 Juan Batiz-Benet
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test daemon command"

. lib/test-lib.sh

gwyport=8082
apiport=5002
swarmport=4003

# TODO: randomize ports here once we add --config to ipfs daemon

# this needs to be in a different test than "ipfs daemon" below
test_expect_success "setup IPFS_PATH" '
  IPFS_PATH="$(pwd)/.ipfs" &&
  export IPFS_PATH
'

api_maddr="/ip4/0.0.0.0/tcp/$apiport"
gway_maddr="/ip4/0.0.0.0/tcp/$gwyport"
test_expect_success 'ipfs init & config' '
  ipfs init &&
  ipfs config Addresses.Gateway $gway_maddr &&
  ipfs config Addresses.API $api_maddr &&
  ipfs config --json=true Addresses.Swarm "[\"/ip4/0.0.0.0/tcp/4003\"]"
'

test_launch_ipfs_daemon

# this errors if we didn't --init $IPFS_PATH correctly
test_expect_success "'ipfs config Identity.PeerID' works" '
  PEERID=$(ipfs config Identity.PeerID)
'

test_expect_success "'ipfs swarm addrs local' works" '
  ipfs swarm addrs local >local_addrs
'

test_expect_success "ipfs peer id looks good" '
  test_check_peerid "$PEERID"
'

# this is for checking SetAllowedOrigins race condition for the api and gateway
# See https://github.com/ipfs/go-ipfs/pull/1966
test_expect_success "ipfs API works with the correct allowed origin port" '
  curl -s -X GET -H "Origin:http://localhost:$apiport" -I "http://localhost:$apiport/api/v0/version"
'

test_expect_success "ipfs gateway works with the correct allowed origin port" '
  curl -s -X GET -H "Origin:http://localhost:$gwyport" -I "http://localhost:$gwyport/api/v0/version"
'

test_expect_success "ipfs daemon output looks good" '
  STARTFILE="ipfs cat /ipfs/$HASH_WELCOME_DOCS/readme" &&
  echo "Initializing daemon..." >expected_daemon &&
  sed "s/^/Swarm listening on /" local_addrs >>expected_daemon &&
  echo "API server listening on '$API_MADDR'" >>expected_daemon &&
  echo "Gateway (readonly) server listening on '$GWAY_MADDR'" >>expected_daemon &&
  echo "Daemon is ready" >>expected_daemon &&
  test_cmp expected_daemon actual_daemon
'

test_expect_success ".ipfs/ has been created" '
  test -d ".ipfs" &&
  test -f ".ipfs/config" &&
  test -d ".ipfs/datastore" &&
  test -d ".ipfs/blocks" ||
  test_fsh ls -al .ipfs
'

# begin same as in t0010

test_expect_success "ipfs version succeeds" '
	ipfs version >version.txt
'

test_expect_success "ipfs version output looks good" '
	egrep "^ipfs version [0-9]+\.[0-9]+\.[0-9]" version.txt >/dev/null ||
	test_fsh cat version.txt
'

test_expect_success "ipfs help succeeds" '
	ipfs help >help.txt
'

test_expect_success "ipfs help output looks good" '
	egrep -i "^Usage:" help.txt >/dev/null &&
	egrep "ipfs .* <command>" help.txt >/dev/null ||
	test_fsh cat help.txt
'

# netcat (nc) is needed for the following test
test_expect_success "nc is available" '
	type nc >/dev/null
'

# check transport is encrypted
test_expect_success "transport should be encrypted" '
  nc -w 5 localhost '$swarmport' >swarmnc &&
  grep -q "AES-256,AES-128" swarmnc &&
  test_must_fail grep -q "/multistream/1.0.0" swarmnc ||
	test_fsh cat swarmnc
'

test_expect_success "output from streaming commands works" '
	test_expect_code 28 curl -m 2 http://localhost:'$apiport'/api/v0/stats/bw\?poll=true > statsout
'

test_expect_success "output looks good" '
	grep TotalIn statsout > /dev/null &&
	grep TotalOut statsout > /dev/null &&
	grep RateIn statsout > /dev/null &&
	grep RateOut statsout >/dev/null
'

# end same as in t0010

test_expect_success "daemon is still running" '
  kill -0 $IPFS_PID
'

test_expect_success "'ipfs daemon' can be killed" '
  test_kill_repeat_10_sec $IPFS_PID
'

test_expect_success "'ipfs daemon' should be able to run with a pipe attached to stdin (issue #861)" '
  yes | ipfs daemon --init >stdin_daemon_out 2>stdin_daemon_err &
  pollEndpoint -host='$API_MADDR' -ep=/version -v -tout=1s -tries=10 >stdin_poll_apiout 2>stdin_poll_apierr &&
  test_kill_repeat_10_sec $! ||
  test_fsh cat stdin_daemon_out || test_fsh cat stdin_daemon_err || test_fsh cat stdin_poll_apiout || test_fsh cat stdin_poll_apierr
'

test_done
