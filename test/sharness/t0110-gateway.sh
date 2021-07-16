#!/usr/bin/env bash
#
# Copyright (c) 2015 Matt Bell
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test HTTP Gateway"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon

port=$GWAY_PORT
apiport=$API_PORT

# TODO check both 5001 and 5002.
# 5001 should have a readable gateway (part of the API)
# 5002 should have a readable gateway (using ipfs config Addresses.Gateway)
# but ideally we should only write the tests once. so maybe we need to
# define a function to test a gateway, and do so for each port.
# for now we check 5001 here as 5002 will be checked in gateway-writable.

test_expect_success "Make a file to test with" '
  echo "Hello Worlds!" >expected &&
  HASH=$(ipfs add -q expected) ||
  test_fsh cat daemon_err
'

test_expect_success "GET IPFS path succeeds" '
  curl -sfo actual "http://127.0.0.1:$port/ipfs/$HASH"
'

test_expect_success "GET IPFS path with explicit ?filename succeeds with proper header" "
  curl -fo actual -D actual_headers 'http://127.0.0.1:$port/ipfs/$HASH?filename=testтест.pdf' &&
  grep -F 'Content-Disposition: inline; filename=\"test____.pdf\"; filename*=UTF-8'\'\''test%D1%82%D0%B5%D1%81%D1%82.pdf' actual_headers
"

test_expect_success "GET IPFS path with explicit ?filename and &download=true succeeds with proper header" "
  curl -fo actual -D actual_headers 'http://127.0.0.1:$port/ipfs/$HASH?filename=testтест.mp4&download=true' &&
  grep -F 'Content-Disposition: attachment; filename=\"test____.mp4\"; filename*=UTF-8'\'\''test%D1%82%D0%B5%D1%81%D1%82.mp4' actual_headers
"

# https://github.com/ipfs/go-ipfs/issues/4025#issuecomment-342250616
test_expect_success "GET for Service Worker registration outside of an IPFS content root errors" "
  curl -H 'Service-Worker: script'  -svX GET 'http://127.0.0.1:$port/ipfs/$HASH?filename=sw.js' > curl_sw_out 2>&1 &&
  grep 'HTTP/1.1 400 Bad Request' curl_sw_out &&
  grep 'navigator.serviceWorker: registration is not allowed for this scope' curl_sw_out
"

test_expect_success "GET IPFS path output looks good" '
  test_cmp expected actual &&
  rm actual
'

test_expect_success "GET IPFS directory path succeeds" '
  mkdir -p dir/dirwithindex &&
  echo "12345" >dir/test &&
  echo "hello i am a webpage" >dir/dirwithindex/index.html &&
  ipfs add -r -q dir >actual &&
  HASH2=$(tail -n 1 actual) &&
  curl -sf "http://127.0.0.1:$port/ipfs/$HASH2"
'

test_expect_success "GET IPFS directory file succeeds" '
  curl -sfo actual "http://127.0.0.1:$port/ipfs/$HASH2/test"
'

test_expect_success "GET IPFS directory file output looks good" '
  test_cmp dir/test actual
'

test_expect_success "GET IPFS directory with index.html returns redirect to add trailing slash" "
  curl -sI -o response_without_slash \"http://127.0.0.1:$port/ipfs/$HASH2/dirwithindex?query=to-remember\"  &&
  test_should_contain \"Location: /ipfs/$HASH2/dirwithindex/?query=to-remember\" response_without_slash
"

test_expect_success "GET IPFS directory with index.html and trailing slash returns expected output" "
  curl -s -o response_with_slash \"http://127.0.0.1:$port/ipfs/$HASH2/dirwithindex/?query=to-remember\"  &&
  test_should_contain \"hello i am a webpage\" response_with_slash
"

test_expect_success "GET IPFS nonexistent file returns code expected (404)" '
  test_curl_resp_http_code "http://127.0.0.1:$port/ipfs/$HASH2/pleaseDontAddMe" "HTTP/1.1 404 Not Found"
'

# https://github.com/ipfs/go-ipfs/issues/8230
test_expect_success "GET IPFS inlined zero-length data object returns ok code (200)" '
  curl -sD - "http://127.0.0.1:$port/ipfs/bafkqaaa" > empty_ok_response &&
  test_should_contain "HTTP/1.1 200 OK" empty_ok_response &&
  test_should_contain "Content-Length: 0" empty_ok_response
'

test_expect_success "GET /ipfs/ipfs/{cid} returns redirect to the valid path" '
  curl -sD - "http://127.0.0.1:$port/ipfs/ipfs/bafkqaaa?query=to-remember" > response_with_double_ipfs_ns &&
  test_should_contain "<meta http-equiv=\"refresh\" content=\"10;url=/ipfs/bafkqaaa?query=to-remember\" />" response_with_double_ipfs_ns &&
  test_should_contain "<link rel=\"canonical\" href=\"/ipfs/bafkqaaa?query=to-remember\" />" response_with_double_ipfs_ns
'

test_expect_failure "GET IPNS path succeeds" '
  ipfs name publish --allow-offline "$HASH" &&
  PEERID=$(ipfs config Identity.PeerID) &&
  test_check_peerid "$PEERID" &&
  curl -sfo actual "http://127.0.0.1:$port/ipns/$PEERID"
'

test_expect_failure "GET IPNS path output looks good" '
  test_cmp expected actual
'

test_expect_success "GET /ipfs/ipns/{peerid} returns redirect to the valid path" '
  PEERID=$(ipfs config Identity.PeerID) &&
  curl -sD - "http://127.0.0.1:$port/ipfs/ipns/${PEERID}?query=to-remember" > response_with_ipfs_ipns_ns &&
  test_should_contain "<meta http-equiv=\"refresh\" content=\"10;url=/ipns/${PEERID}?query=to-remember\" />" response_with_ipfs_ipns_ns &&
  test_should_contain "<link rel=\"canonical\" href=\"/ipns/${PEERID}?query=to-remember\" />" response_with_ipfs_ipns_ns
'

test_expect_success "GET invalid IPFS path errors" '
  test_must_fail curl -sf "http://127.0.0.1:$port/ipfs/12345"
'

test_expect_success "GET invalid path errors" '
  test_must_fail curl -sf "http://127.0.0.1:$port/12345"
'

test_expect_success "GET /webui returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /webui/ returns code expected" '
  test_curl_resp_http_code "http://127.0.0.1:$apiport/webui/" "HTTP/1.1 302 Found" "HTTP/1.1 301 Moved Permanently"
'

test_expect_success "GET /logs returns logs" '
  test_expect_code 28 curl http://127.0.0.1:$apiport/logs -m1 > log_out
'

test_expect_success "log output looks good" '
  grep "log API client connected" log_out
'

test_expect_success "GET /api/v0/version succeeds" '
  curl -X POST -v "http://127.0.0.1:$apiport/api/v0/version" 2> version_out
'

test_expect_success "output only has one transfer encoding header" '
  grep "Transfer-Encoding: chunked" version_out | wc -l | xargs echo > tecount_out &&
  echo "1" > tecount_exp &&
  test_cmp tecount_out tecount_exp
'

curl_pprofmutex() {
  curl -f -X POST "http://127.0.0.1:$apiport/debug/pprof-mutex/?fraction=$1"
}

test_expect_success "set mutex fraction for pprof (negative so it doesn't enable)" '
  curl_pprofmutex -1
'

test_expect_success "test failure conditions of mutex pprof endpoint" '
  test_must_fail curl_pprofmutex &&
    test_must_fail curl_pprofmutex that_is_string &&
    test_must_fail curl -f -X GET "http://127.0.0.1:$apiport/debug/pprof-mutex/?fraction=-1"
'


test_expect_success "setup index hash" '
  mkdir index &&
  echo "<p></p>" > index/index.html &&
  INDEXHASH=$(ipfs add -q -r index | tail -n1)
  echo index: $INDEXHASH
'

test_expect_success "GET 'index.html' has correct content type" '
  curl -I "http://127.0.0.1:$port/ipfs/$INDEXHASH/" > indexout
'

test_expect_success "output looks good" '
  grep "Content-Type: text/html" indexout
'

test_expect_success "HEAD 'index.html' has no content" '
  curl -X HEAD --max-time 1 http://127.0.0.1:$port/ipfs/$INDEXHASH/ > output;
  [ ! -s output ]
'

# test ipfs readonly api

test_curl_gateway_api() {
  curl -sfo actual "http://127.0.0.1:$port/api/v0/$1"
}

test_expect_success "get IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "cat?arg=$HASH2/test"
'

test_expect_success "get IPFS directory file through readonly API output looks good" '
  test_cmp dir/test actual
'

test_expect_success "refs IPFS directory file through readonly API succeeds" '
  test_curl_gateway_api "refs?arg=$HASH2/test"
'

for cmd in add  \
           block/put \
           bootstrap \
           config \
           dag/put \
           dag/import \
           dht \
           diag \
           id \
           mount \
           name/publish \
           object/put \
           object/new \
           object/patch \
           pin \
           ping \
           repo \
           stats \
           swarm \
           file \
           update \
           bitswap
do
  test_expect_success "test gateway api is sanitized: $cmd" '
    test_curl_resp_http_code "http://127.0.0.1:$port/api/v0/$cmd" "HTTP/1.1 404 Not Found"
  '
done

# This one is different. `local` will be interpreted as a path if the command isn't defined.
test_expect_success "test gateway api is sanitized: refs/local" '
    echo "Error: invalid path \"local\": selected encoding not supported" > refs_local_expected &&
    ! ipfs --api /ip4/127.0.0.1/tcp/$port refs local > refs_local_actual 2>&1 &&
    test_cmp refs_local_expected refs_local_actual
  '

test_expect_success "create raw-leaves node" '
  echo "This is RAW!" > rfile &&
  echo "This is RAW!" | ipfs add --raw-leaves -q > rhash
'

test_expect_success "try fetching it from gateway" '
  curl http://127.0.0.1:$port/ipfs/$(cat rhash) > ffile &&
  test_cmp rfile ffile
'

test_expect_success "Add compact blocks" '
  ipfs block put ../t0110-gateway-data/foo.block &&
  FOO2_HASH=$(ipfs block put ../t0110-gateway-data/foofoo.block) &&
  printf "foofoo" > expected
'

test_expect_success "GET compact blocks succeeds" '
  curl -o actual "http://127.0.0.1:$port/ipfs/$FOO2_HASH" &&
  test_cmp expected actual
'

test_kill_ipfs_daemon


GWPORT=32563

test_expect_success "set up iptb testbed" '
  iptb testbed create -type localipfs -count 5 -force -init &&
  ipfsi 0 config Addresses.Gateway /ip4/127.0.0.1/tcp/$GWPORT &&
  PEERID_1=$(iptb attr get 1 id)
'

test_expect_success "set NoFetch to true in config of node 0" '
  ipfsi 0 config --bool=true Gateway.NoFetch true
'

test_expect_success "start ipfs nodes" '
  iptb start -wait &&
  iptb connect 0 1
'

test_expect_success "try fetching not present key from node 0" '
  FOO=$(echo "foo" | ipfsi 1 add -Q) &&
  test_expect_code 22 curl -f "http://127.0.0.1:$GWPORT/ipfs/$FOO"
'

test_expect_success "try fetching not present ipns key from node 0" '
  ipfsi 1 name publish /ipfs/$FOO &&
  test_expect_code 22 curl -f "http://127.0.0.1:$GWPORT/ipns/$PEERID_1"
'

test_expect_success "try fetching present key from node 0" '
  BAR=$(echo "bar" | ipfsi 0 add -Q) &&
  curl -f "http://127.0.0.1:$GWPORT/ipfs/$BAR"
'

test_expect_success "try fetching present ipns key from node 0" '
  ipfsi 1 name publish /ipfs/$BAR &&
  curl "http://127.0.0.1:$GWPORT/ipns/$PEERID_1"
'

test_expect_success "stop testbed" '
  iptb stop
'

test_done
