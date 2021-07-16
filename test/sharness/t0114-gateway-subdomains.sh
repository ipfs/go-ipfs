#!/usr/bin/env bash
#
# Copyright (c) Protocol Labs

test_description="Test subdomain support on the HTTP gateway"


. lib/test-lib.sh

## ============================================================================
## Helpers specific to subdomain tests
## ============================================================================

# Helper that tests gateway response over direct HTTP
# and in all supported HTTP proxy modes
test_localhost_gateway_response_should_contain() {
  local label="$1"
  local expected="$3"

  # explicit "Host: $hostname" header to match browser behavior
  # and also make tests independent from DNS
  local host=$(echo $2 | cut -d'/' -f3 | cut -d':' -f1)
  local hostname=$(echo $2 | cut -d'/' -f3 | cut -d':' -f1,2)

  # Proxy is the same as HTTP Gateway, we use raw IP and port to be sure
  local proxy="http://127.0.0.1:$GWAY_PORT"

  # Create a raw URL version with IP to ensure hostname from Host header is used
  # (removes false-positives, Host header is used for passing hostname already)
  local url="$2"
  local rawurl=$(echo "$url" | sed "s/$hostname/127.0.0.1:$GWAY_PORT/")

  #echo "hostname:   $hostname"
  #echo "url before: $url"
  #echo "url after:  $rawurl"

  # regular HTTP request
  # (hostname in Host header, raw IP in URL)
  test_expect_success "$label (direct HTTP)" "
    curl -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy
  # (hostname is passed via URL)
  # Note: proxy client should not care, but curl does DNS lookup
  # for some reason anyway, so we pass static DNS mapping
  test_expect_success "$label (HTTP proxy)" "
    curl -x $proxy --resolve $hostname:127.0.0.1 -sD - \"$url\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy 1.0
  # (repeating proxy test with older spec, just to be sure)
  test_expect_success "$label (HTTP proxy 1.0)" "
    curl --proxy1.0 $proxy --resolve $hostname:127.0.0.1 -sD - \"$url\" > response &&
    test_should_contain \"$expected\" response
  "

  # HTTP proxy tunneling (CONNECT)
  # https://tools.ietf.org/html/rfc7231#section-4.3.6
  # In HTTP/1.x, the pseudo-method CONNECT
  # can be used to convert an HTTP connection into a tunnel to a remote host
  test_expect_success "$label (HTTP proxy tunneling)" "
    curl --proxytunnel -x $proxy -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "
}

# Helper that checks gateway response for specific hostname in Host header
test_hostname_gateway_response_should_contain() {
  local label="$1"
  local hostname="$2"
  local url="$3"
  local rawurl=$(echo "$url" | sed "s/$hostname/127.0.0.1:$GWAY_PORT/")
  local expected="$4"
  test_expect_success "$label" "
    curl -H \"Host: $hostname\" -sD - \"$rawurl\" > response &&
    test_should_contain \"$expected\" response
  "
}

## ============================================================================
## Start IPFS Node and prepare test CIDs
## ============================================================================

test_expect_success "ipfs init" '
  export IPFS_PATH="$(pwd)/.ipfs" &&
  ipfs init --profile=test > /dev/null
'

test_launch_ipfs_daemon_without_network

# CIDv0to1 is necessary because raw-leaves are enabled by default during
# "ipfs add" with CIDv1 and disabled with CIDv0
test_expect_success "Add test text file" '
  CID_VAL="hello"
  CIDv1=$(echo $CID_VAL | ipfs add --cid-version 1 -Q)
  CIDv0=$(echo $CID_VAL | ipfs add --cid-version 0 -Q)
  CIDv0to1=$(echo "$CIDv0" | ipfs cid base32)
  echo CIDv0to1=${CIDv0to1}
'

# Directory tree crafted to test for edge cases like "/ipfs/ipfs/ipns/bar"
test_expect_success "Add the test directory" '
  mkdir -p testdirlisting/ipfs/ipns &&
  echo "hello" > testdirlisting/hello &&
  echo "text-file-content" > testdirlisting/ipfs/ipns/bar &&
  mkdir -p testdirlisting/api &&
  mkdir -p testdirlisting/ipfs &&
  echo "I am a txt file" > testdirlisting/api/file.txt &&
  echo "I am a txt file" > testdirlisting/ipfs/file.txt &&
  DIR_CID=$(ipfs add -Qr --cid-version 1 testdirlisting)
'

test_expect_success "Publish test text file to IPNS using RSA keys" '
  RSA_KEY=$(ipfs key gen --ipns-base=b58mh --type=rsa --size=2048 test_key_rsa | head -n1 | tr -d "\n")
  RSA_IPNS_IDv0=$(echo "$RSA_KEY" | ipfs cid format -v 0)
  RSA_IPNS_IDv1=$(echo "$RSA_KEY" | ipfs cid format -v 1 --codec libp2p-key -b base36)
  RSA_IPNS_IDv1_DAGPB=$(echo "$RSA_IPNS_IDv0" | ipfs cid format -v 1 -b base36)
  test_check_peerid "${RSA_KEY}" &&
  ipfs name publish --key test_key_rsa --allow-offline -Q "/ipfs/$CIDv1" > name_publish_out &&
  ipfs name resolve "$RSA_KEY"  > output &&
  printf "/ipfs/%s\n" "$CIDv1" > expected2 &&
  test_cmp expected2 output
'

test_expect_success "Publish test text file to IPNS using ED25519 keys" '
  ED25519_KEY=$(ipfs key gen --ipns-base=b58mh --type=ed25519 test_key_ed25519 | head -n1 | tr -d "\n")
  ED25519_IPNS_IDv0=$ED25519_KEY
  ED25519_IPNS_IDv1=$(ipfs key list -l --ipns-base=base36 | grep test_key_ed25519 | cut -d " " -f1 | tr -d "\n")
  ED25519_IPNS_IDv1_DAGPB=$(echo "$ED25519_IPNS_IDv1" | ipfs cid format -v 1 -b base36 --codec protobuf)
  test_check_peerid "${ED25519_KEY}" &&
  ipfs name publish --key test_key_ed25519 --allow-offline -Q "/ipfs/$CIDv1" > name_publish_out &&
  ipfs name resolve "$ED25519_KEY"  > output &&
  printf "/ipfs/%s\n" "$CIDv1" > expected2 &&
  test_cmp expected2 output
'

# ensure we start with empty Gateway.PublicGateways
test_expect_success 'start daemon with empty config for Gateway.PublicGateways' '
  test_kill_ipfs_daemon &&
  ipfs config --json Gateway.PublicGateways "{}" &&
  test_launch_ipfs_daemon_without_network
'

## ============================================================================
## Test path-based requests to a local gateway with default config
## (forced redirects to http://*.localhost)
## ============================================================================

# /ipfs/<cid>

# IP remains old school path-based gateway

test_localhost_gateway_response_should_contain \
  "request for 127.0.0.1/ipfs/{CID} stays on path" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# 'localhost' hostname is used for subdomains, and should not return
#  payload directly, but redirect to URL with proper origin isolation

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} returns HTTP 301 Moved Permanently" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "301 Moved Permanently"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} returns Location HTTP header for subdomain redirect in browsers" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.localhost:$GWAY_PORT/"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{DIR_CID} returns HTTP 301 Moved Permanently" \
  "http://localhost:$GWAY_PORT/ipfs/$DIR_CID" \
  "301 Moved Permanently"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{DIR_CID} returns Location HTTP header for subdomain redirect in browsers" \
  "http://localhost:$GWAY_PORT/ipfs/$DIR_CID/" \
  "Location: http://$DIR_CID.ipfs.localhost:$GWAY_PORT/"

# We return body with HTTP 301 so existing cli scripts that use path-based
# gateway do not break (curl doesn't auto-redirect without passing -L; wget
# does not span across hostnames by default)
# Context: https://github.com/ipfs/go-ipfs/issues/6975
test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv1} includes valid payload in body for CLI tools like curl" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CIDv0} redirects to CIDv1 representation in subdomain" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.localhost:$GWAY_PORT/"

# /ipns/<libp2p-key>

test_localhost_gateway_response_should_contain \
  "request for localhost/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/$RSA_IPNS_IDv0" \
  "Location: http://${RSA_IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

test_localhost_gateway_response_should_contain \
  "request for localhost/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/$ED25519_IPNS_IDv0" \
  "Location: http://${ED25519_IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# /ipns/<dnslink-fqdn>

test_localhost_gateway_response_should_contain \
  "request for localhost/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "http://localhost:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.localhost:$GWAY_PORT/wiki"

# API on localhost subdomain gateway

# /api/v0 present on the root hostname
test_localhost_gateway_response_should_contain \
  "request for localhost/api" \
  "http://localhost:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "Ref"

# /api/v0 not mounted on content root subdomains
test_localhost_gateway_response_should_contain \
  "request for {cid}.ipfs.localhost/api returns data if present on the content root" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

test_localhost_gateway_response_should_contain \
  "request for {cid}.ipfs.localhost/api/v0/refs returns 404" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

## ============================================================================
## Test subdomain-based requests to a local gateway with default config
## (origin per content root at http://*.localhost)
## ============================================================================

# {CID}.ipfs.localhost

test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost should return expected payload" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT" \
  "$CID_VAL"

# ensure /ipfs/ namespace is not mounted on subdomain
test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost/ipfs/{CID} should return HTTP 404" \
  "http://${CIDv1}.ipfs.localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# ensure requests to /ipfs/* are not blocked, if content root has such subdirectory
test_localhost_gateway_response_should_contain \
  "request for {CID}.ipfs.localhost/ipfs/file.txt should return data from a file in CID content root" \
  "http://${DIR_CID}.ipfs.localhost:$GWAY_PORT/ipfs/file.txt" \
  "I am a txt file"

# {CID}.ipfs.localhost/sub/dir (Directory Listing)
DIR_HOSTNAME="${DIR_CID}.ipfs.localhost:$GWAY_PORT"

test_expect_success "valid file and subdirectory paths in directory listing at {cid}.ipfs.localhost" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME" > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/ipfs\">ipfs</a>" list_response
'

test_expect_success "valid parent directory path in directory listing at {cid}.ipfs.localhost/sub/dir" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME/ipfs/ipns/" > list_response &&
  test_should_contain "<a href=\"/ipfs/ipns/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/ipfs/ipns/bar\">bar</a>" list_response
'

test_expect_success "request for deep path resource at {cid}.ipfs.localhost/sub/dir/file" '
  curl -s --resolve $DIR_HOSTNAME:127.0.0.1 "http://$DIR_HOSTNAME/ipfs/ipns/bar" > list_response &&
  test_should_contain "text-file-content" list_response
'


# *.ipns.localhost

# <libp2p-key>.ipns.localhost

test_localhost_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.localhost returns expected payload" \
  "http://${RSA_IPNS_IDv1}.ipns.localhost:$GWAY_PORT" \
  "$CID_VAL"

test_localhost_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.localhost returns expected payload" \
  "http://${ED25519_IPNS_IDv1}.ipns.localhost:$GWAY_PORT" \
  "$CID_VAL"

test_localhost_gateway_response_should_contain \
  "localhost request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "http://${RSA_IPNS_IDv1_DAGPB}.ipns.localhost:$GWAY_PORT" \
  "Location: http://${RSA_IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

test_localhost_gateway_response_should_contain \
  "localhost request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "http://${ED25519_IPNS_IDv1_DAGPB}.ipns.localhost:$GWAY_PORT" \
  "Location: http://${ED25519_IPNS_IDv1}.ipns.localhost:$GWAY_PORT/"

# <dnslink-fqdn>.ipns.localhost

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
test_kill_ipfs_daemon
DNSLINK_FQDN="dnslink-test.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"
test_launch_ipfs_daemon

test_localhost_gateway_response_should_contain \
  "request for {dnslink}.ipns.localhost returns expected payload" \
  "http://$DNSLINK_FQDN.ipns.localhost:$GWAY_PORT" \
  "$CID_VAL"

# api.localhost/api

# Note: we use DIR_CID so refs -r returns some CIDs for child nodes
test_localhost_gateway_response_should_contain \
  "request for api.localhost returns API response" \
  "http://api.localhost:$GWAY_PORT/api/v0/refs?arg=$DIR_CID&r=true" \
  "Ref"

## ============================================================================
## Test subdomain-based requests with a custom hostname config
## (origin per content root at http://*.example.com)
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs", "/ipns", "/api"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network


# example.com/ip(f|n)s/*
# =============================================================================

# path requests to the root hostname should redirect
# to a subdomain URL with proper origin isolation

test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.example.com/"

# error message should include original CID
# (and it should be case-sensitive, as we can't assume everyone uses base32)
test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{InvalidCID} produces useful error before redirect" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/QmInvalidCID" \
  'invalid path \"/ipfs/QmInvalidCID\"'

test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv0} produces redirect to {CIDv1}.ipfs.example.com" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv0" \
  "Location: http://${CIDv0to1}.ipfs.example.com/"

# Support X-Forwarded-Proto
test_expect_success "request for http://example.com/ipfs/{CID} with X-Forwarded-Proto: https produces redirect to HTTPS URL" "
  curl -H \"X-Forwarded-Proto: https\" -H \"Host: example.com\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1\" > response &&
  test_should_contain \"Location: https://$CIDv1.ipfs.example.com/\" response
"

# Support ipfs:// in https://developer.mozilla.org/en-US/docs/Web/API/Navigator/registerProtocolHandler
test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/?uri=ipfs%3A%2F%2F.. produces redirect to /ipfs/.. content path" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/?uri=ipfs%3A%2F%2FQmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco%2Fwiki%2FDiego_Maradona.html" \
  "Location: /ipfs/QmXoypizjW3WknFiJnKLwHCnL72vedxjQkDDP1mXWo6uco/wiki/Diego_Maradona.html"

# example.com/ipns/<libp2p-key>

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$RSA_IPNS_IDv0" \
  "Location: http://${RSA_IPNS_IDv1}.ipns.example.com/"

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/{CIDv0} redirects to CIDv1 with libp2p-key multicodec in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$ED25519_IPNS_IDv0" \
  "Location: http://${ED25519_IPNS_IDv1}.ipns.example.com/"

# example.com/ipns/<dnslink-fqdn>

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/{fqdn} redirects to DNSLink in subdomain" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki" \
  "Location: http://en.wikipedia-on-ipfs.org.ipns.example.com/wiki"

# DNSLink on Public gateway with a single-level wildcard TLS cert
# "Option C" from  https://github.com/ipfs/in-web-browsers/issues/169
test_expect_success \
  "request for example.com/ipns/{fqdn} with X-Forwarded-Proto redirects to TLS-safe label in subdomain" "
  curl -H \"Host: example.com\" -H \"X-Forwarded-Proto: https\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipns/en.wikipedia-on-ipfs.org/wiki\" > response &&
  test_should_contain \"Location: https://en-wikipedia--on--ipfs-org.ipns.example.com/wiki\" response
  "

# Support ipns:// in https://developer.mozilla.org/en-US/docs/Web/API/Navigator/registerProtocolHandler
test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/?uri=ipns%3A%2F%2F.. produces redirect to /ipns/.. content path" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/?uri=ipns%3A%2F%2Fen.wikipedia-on-ipfs.org" \
  "Location: /ipns/en.wikipedia-on-ipfs.org"

# *.ipfs.example.com: subdomain requests made with custom FQDN in Host header

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com should return expected payload" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 404" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# {CID}.ipfs.example.com/sub/dir (Directory Listing)
DIR_FQDN="${DIR_CID}.ipfs.example.com"

test_expect_success "valid file and directory paths in directory listing at {cid}.ipfs.example.com" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT > list_response &&
  test_should_contain "<a href=\"/hello\">hello</a>" list_response &&
  test_should_contain "<a href=\"/ipfs\">ipfs</a>" list_response
'

test_expect_success "valid parent directory path in directory listing at {cid}.ipfs.example.com/sub/dir" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT/ipfs/ipns/ > list_response &&
  test_should_contain "<a href=\"/ipfs/ipns/./..\">..</a>" list_response &&
  test_should_contain "<a href=\"/ipfs/ipns/bar\">bar</a>" list_response
'

# Note 1: we test for sneaky subdir names  {cid}.ipfs.example.com/ipfs/ipns/ :^)
# Note 2: example.com/ipfs/.. present in HTML will be redirected to subdomain, so this is expected behavior
test_expect_success "valid breadcrumb links in the header of directory listing at {cid}.ipfs.example.com/sub/dir" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT/ipfs/ipns/ > list_response &&
  test_should_contain "Index of" list_response &&
  test_should_contain "/ipfs/<a href=\"//example.com/ipfs/${DIR_CID}\">${DIR_CID}</a>/<a href=\"//example.com/ipfs/${DIR_CID}/ipfs\">ipfs</a>/<a href=\"//example.com/ipfs/${DIR_CID}/ipfs/ipns\">ipns</a>" list_response
'

test_expect_success "request for deep path resource {cid}.ipfs.example.com/sub/dir/file" '
  curl -s -H "Host: $DIR_FQDN" http://127.0.0.1:$GWAY_PORT/ipfs/ipns/bar > list_response &&
  test_should_contain "text-file-content" list_response
'

# *.ipns.example.com
# ============================================================================

# <libp2p-key>.ipns.example.com

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${RSA_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {CIDv1-libp2p-key}.ipns.example.com returns expected payload" \
  "${ED25519_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "hostname request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${RSA_IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${RSA_IPNS_IDv1}.ipns.example.com/"

test_hostname_gateway_response_should_contain \
  "hostname request for {CIDv1-dag-pb}.ipns.localhost redirects to CID with libp2p-key multicodec" \
  "${ED25519_IPNS_IDv1_DAGPB}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "Location: http://${ED25519_IPNS_IDv1}.ipns.example.com/"

# API on subdomain gateway example.com
# ============================================================================

# present at the root domain
test_hostname_gateway_response_should_contain \
  "request for example.com/api/v0/refs returns expected payload when /api is on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "Ref"

# not mounted on content root subdomains
test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api returns data if present on the content root" \
  "$DIR_CID.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api/v0/refs returns 404" \
  "$CIDv1.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

# disable /api on example.com
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs", "/ipns"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

# not mounted at the root domain
test_hostname_gateway_response_should_contain \
  "request for example.com/api/v0/refs returns 404 if /api not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/v0/refs?arg=${DIR_CID}&r=true" \
  "404 Not Found"

# not mounted on content root subdomains
test_hostname_gateway_response_should_contain \
  "request for {cid}.ipfs.example.com/api returns data if present on the content root" \
  "$DIR_CID.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/api/file.txt" \
  "I am a txt file"

# DNSLink: <dnslink-fqdn>.ipns.example.com
# (not really useful outside of localhost, as setting TLS for more than one
# level of wildcard is a pain, but we support it if someone really wants it)
# ============================================================================

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
test_kill_ipfs_daemon
DNSLINK_FQDN="dnslink-subdomain-gw-test.example.org"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"
test_launch_ipfs_daemon

test_hostname_gateway_response_should_contain \
  "request for {dnslink}.ipns.example.com returns expected payload" \
  "$DNSLINK_FQDN.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "$CID_VAL"

# DNSLink on Public gateway with a single-level wildcard TLS cert
# "Option C" from  https://github.com/ipfs/in-web-browsers/issues/169
test_expect_success \
  "request for {single-label-dnslink}.ipns.example.com with X-Forwarded-Proto returns expected payload" "
  curl -H \"Host: dnslink--subdomain--gw--test-example-org.ipns.example.com\" -H \"X-Forwarded-Proto: https\" -sD - \"http://127.0.0.1:$GWAY_PORT\" > response &&
  test_should_contain \"$CID_VAL\" response
  "

## Test subdomain handling of CIDs that do not fit in a single DNS Label (>63chars)
## https://github.com/ipfs/go-ipfs/issues/7318
## ============================================================================

# ed25519 fits under 63 char limit when represented in base36
IPNS_KEY="test_key_ed25519"
IPNS_ED25519_B58MH=$(ipfs key list -l --ipns-base b58mh | grep $IPNS_KEY | cut -d" " -f1 | tr -d "\n")
IPNS_ED25519_B36CID=$(ipfs key list -l --ipns-base base36 | grep $IPNS_KEY | cut -d" " -f1 | tr -d "\n")
# sha512 will be over 63char limit, even when represented in Base36
CIDv1_TOO_LONG=$(echo $CID_VAL | ipfs add --cid-version 1 --hash sha2-512 -Q)

# local: *.localhost
test_localhost_gateway_response_should_contain \
  "request for a ED25519 libp2p-key at localhost/ipns/{b58mh} returns Location HTTP header for DNS-safe subdomain redirect in browsers" \
  "http://localhost:$GWAY_PORT/ipns/$IPNS_ED25519_B58MH" \
  "Location: http://${IPNS_ED25519_B36CID}.ipns.localhost:$GWAY_PORT/"

# router should not redirect to hostnames that could fail due to DNS limits
test_localhost_gateway_response_should_contain \
  "request for a too long CID at localhost/ipfs/{CIDv1} returns human readable error" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1_TOO_LONG" \
  "CID incompatible with DNS label length limit of 63"

test_localhost_gateway_response_should_contain \
  "request for a too long CID at localhost/ipfs/{CIDv1} returns HTTP Error 400 Bad Request" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1_TOO_LONG" \
  "400 Bad Request"

# direct request should also fail (provides the same UX as router and avoids confusion)
test_localhost_gateway_response_should_contain \
  "request for a too long CID at {CIDv1}.ipfs.localhost returns expected payload" \
  "http://$CIDv1_TOO_LONG.ipfs.localhost:$GWAY_PORT" \
  "400 Bad Request"

# public subdomain gateway: *.example.com

test_hostname_gateway_response_should_contain \
  "request for a ED25519 libp2p-key at example.com/ipns/{b58mh} returns Location HTTP header for DNS-safe subdomain redirect in browsers" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$IPNS_ED25519_B58MH" \
  "Location: http://${IPNS_ED25519_B36CID}.ipns.example.com"

test_hostname_gateway_response_should_contain \
  "request for a too long CID at example.com/ipfs/{CIDv1} returns human readable error" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1_TOO_LONG" \
  "CID incompatible with DNS label length limit of 63"

test_hostname_gateway_response_should_contain \
  "request for a too long CID at example.com/ipfs/{CIDv1} returns HTTP Error 400 Bad Request" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1_TOO_LONG" \
  "400 Bad Request"

test_hostname_gateway_response_should_contain \
  "request for a too long CID at {CIDv1}.ipfs.example.com returns HTTP Error 400 Bad Request" \
  "$CIDv1_TOO_LONG.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "400 Bad Request"

# Disable selected Paths for the subdomain gateway hostname
# =============================================================================

# disable /ipns for the hostname by not whitelisting it
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "request for *.ipns.example.com returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "${RSA_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for *.ipns.example.com returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "${ED25519_IPNS_IDv1}.ipns.example.com" \
  "http://127.0.0.1:$GWAY_PORT" \
  "404 Not Found"

## ============================================================================
## Test path-based requests with a custom hostname config
## ============================================================================

# set explicit no-subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  }
}' || exit 1

# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

# example.com/ip(f|n)s/* smoke-tests
# =============================================================================

# confirm path gateway works for /ipfs
test_hostname_gateway_response_should_contain \
  "request for example.com/ipfs/{CIDv1} returns expected payload" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# refuse subdomain requests on path gateway
# (we don't want false sense of security)
test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.example.com/ipfs/{CID} should return HTTP 404 Not Found" \
  "${CIDv1}.ipfs.example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "404 Not Found"

# refuse requests to Paths that were not explicitly whitelisted for the hostname
test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/ returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$RSA_IPNS_IDv1" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for example.com/ipns/ returns HTTP 404 Not Found when /ipns is not on Paths whitelist" \
  "example.com" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$ED25519_IPNS_IDv1" \
  "404 Not Found"

## ============================================================================
## Test DNSLink requests with a custom PublicGateway (hostname config)
## (DNSLink site at http://dnslink-test.example.com)
## ============================================================================

test_kill_ipfs_daemon

# disable wildcard DNSLink gateway
# and enable it on specific NSLink hostname
ipfs config --json Gateway.NoDNSLink true && \
ipfs config --json Gateway.PublicGateways '{
  "dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": ["/ipfs"]
  },
  "only-dnslink-enabled-on-fqdn.example.org": {
    "NoDNSLink": false,
    "UseSubdomains": false,
    "Paths": []
  },
  "dnslink-disabled-on-fqdn.example.com": {
    "NoDNSLink": true,
    "UseSubdomains": false,
    "Paths": []
  }
}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
DNSLINK_FQDN="dnslink-enabled-on-fqdn.example.org"
ONLY_DNSLINK_FQDN="only-dnslink-enabled-on-fqdn.example.org"
NO_DNSLINK_FQDN="dnslink-disabled-on-fqdn.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1,$ONLY_DNSLINK_FQDN:/ipfs/$DIR_CID"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CIDv1\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CID_VAL\" result
"

# DNSLink enabled

test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ PublicGateway returns expected payload" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/{cid} returns expected payload when /ipfs is on Paths whitelist" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# Test for a fun edge case: DNSLink-only gateway without  /ipfs/ namespace
# mounted, and with subdirectory named "ipfs" ¯\_(ツ)_/¯
test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/file.txt returns data from content root when /ipfs in not on Paths whitelist" \
  "$ONLY_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/file.txt" \
  "I am a txt file"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipns/{peerid} returns 404 when path is not whitelisted" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$RSA_IPNS_IDv0" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipns/{peerid} returns 404 when path is not whitelisted" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipns/$ED25519_IPNS_IDv0" \
  "404 Not Found"

# DNSLink disabled

test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ returns 404 when NoDNSLink=true" \
  "$NO_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "404 Not Found"

test_hostname_gateway_response_should_contain \
  "request for {dnslink-fqdn}/ipfs/{cid} returns 404 when path is not whitelisted" \
  "$NO_DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv0" \
  "404 Not Found"


## ============================================================================
## Test wildcard DNSLink (any hostname, with default config)
## ============================================================================

test_kill_ipfs_daemon

# enable wildcard DNSLink gateway (any value in Host header)
# and remove custom PublicGateways
ipfs config --json Gateway.NoDNSLink false && \
ipfs config --json Gateway.PublicGateways '{}' || exit 1

# DNSLink test requires a daemon in online mode with precached /ipns/ mapping
DNSLINK_FQDN="wildcard-dnslink-not-in-config.example.com"
export IPFS_NS_MAP="$DNSLINK_FQDN:/ipfs/$CIDv1"

# restart daemon to apply config changes
test_launch_ipfs_daemon

# make sure test setup is valid (fail if CoreAPI is unable to resolve)
test_expect_success "spoofed DNSLink record resolves in cli" "
  ipfs resolve /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CIDv1\" result &&
  ipfs cat /ipns/$DNSLINK_FQDN > result &&
  test_should_contain \"$CID_VAL\" result
"

# gateway test
test_hostname_gateway_response_should_contain \
  "request for http://{dnslink-fqdn}/ (wildcard) returns expected payload" \
  "$DNSLINK_FQDN" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

## ============================================================================
## Test support for X-Forwarded-Host
## ============================================================================

# set explicit subdomain gateway config for the hostname
ipfs config --json Gateway.PublicGateways '{
  "example.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs", "/ipns", "/api"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

test_expect_success "request for http://fake.domain.com/ipfs/{CID} doesn't match the example.com gateway" "
  curl -H \"Host: fake.domain.com\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1\" > response &&
  test_should_contain \"200 OK\" response
"

test_expect_success "request for http://fake.domain.com/ipfs/{CID} with X-Forwarded-Host: example.com match the example.com gateway" "
  curl -H \"Host: fake.domain.com\" -H \"X-Forwarded-Host: example.com\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1\" > response &&
  test_should_contain \"Location: http://$CIDv1.ipfs.example.com/\" response
"

test_expect_success "request for http://fake.domain.com/ipfs/{CID} with X-Forwarded-Host: example.com and X-Forwarded-Proto: https match the example.com gateway, redirect with https" "
  curl -H \"Host: fake.domain.com\" -H \"X-Forwarded-Host: example.com\" -H \"X-Forwarded-Proto: https\" -sD - \"http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1\" > response &&
  test_should_contain \"Location: https://$CIDv1.ipfs.example.com/\" response
"

## ============================================================================
## Test support for wildcards in gateway config
## ============================================================================

# set explicit subdomain gateway config for the hostnames
ipfs config --json Gateway.PublicGateways '{
  "*.example1.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "*.*.example2.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "foo.*.example3.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  },
  "foo.bar-*-boo.example4.com": {
    "UseSubdomains": true,
    "Paths": ["/ipfs"]
  }
}' || exit 1
# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

# *.example1.com

test_hostname_gateway_response_should_contain \
  "request for foo.example1.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.example1.com" \
  "foo.example1.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.example1.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.example1.com should return expected payload" \
  "${CIDv1}.ipfs.foo.example1.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# *.*.example2.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar.example2.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar.example2.com" \
  "foo.bar.example2.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar.example2.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar.example2.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar.example2.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# foo.*.example3.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar.example3.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar.example3.com" \
  "foo.bar.example3.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar.example3.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar.example3.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar.example3.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

# foo.bar-*-boo.example4.com

test_hostname_gateway_response_should_contain \
  "request for foo.bar-dev-boo.example4.com/ipfs/{CIDv1} produces redirect to {CIDv1}.ipfs.foo.bar-dev-boo.example4.com" \
  "foo.bar-dev-boo.example4.com" \
  "http://127.0.0.1:$GWAY_PORT/ipfs/$CIDv1" \
  "Location: http://$CIDv1.ipfs.foo.bar-dev-boo.example4.com/"

test_hostname_gateway_response_should_contain \
  "request for {CID}.ipfs.foo.bar-dev-boo.example4.com should return expected payload" \
  "${CIDv1}.ipfs.foo.bar-dev-boo.example4.com" \
  "http://127.0.0.1:$GWAY_PORT/" \
  "$CID_VAL"

## ============================================================================
## Test support for overriding implicit defaults
## ============================================================================

# disable subdomain gateway at localhost by removing implicit config
ipfs config --json Gateway.PublicGateways '{
  "localhost": null
}' || exit 1

# restart daemon to apply config changes
test_kill_ipfs_daemon
test_launch_ipfs_daemon_without_network

test_localhost_gateway_response_should_contain \
  "request for localhost/ipfs/{CID} stays on path when subdomain gw is explicitly disabled" \
  "http://localhost:$GWAY_PORT/ipfs/$CIDv1" \
  "$CID_VAL"

# =============================================================================
# ensure we end with empty Gateway.PublicGateways
ipfs config --json Gateway.PublicGateways '{}'
test_kill_ipfs_daemon

test_expect_success "clean up ipfs dir" '
  rm -rf "$IPFS_PATH"
'

test_done
