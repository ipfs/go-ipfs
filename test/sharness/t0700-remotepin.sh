#!/usr/bin/env bash

test_description="Test ipfs remote pinning operations"

. lib/test-lib.sh

if [ -z ${DOCKER_HOST+x} ]; then
  # TODO: set up instead of skipping?
  skip_all='Skipping pinning service integration tests: missing DOCKER_HOST, remote pinning service not available'
  test_done
fi

# daemon running in online mode to ensure Pin.origins/PinStatus.delegates work
test_init_ipfs
test_launch_ipfs_daemon

# create user on pinning service
TEST_PIN_SVC="http://${DOCKER_HOST}:5000/api/v1"
TEST_PIN_SVC_KEY=$(curl -s -X POST "$TEST_PIN_SVC/users" -d email="go-ipfs-sharness@ipfs.example.com" | jq --raw-output .access_token)

# pin remote service  add|ls|rm

# confirm empty service list response has proper json struct
# https://github.com/ipfs/go-ipfs/pull/7829
test_expect_success "test 'ipfs pin remote service ls' JSON on empty list" '
  ipfs pin remote service ls --stat --enc=json | tee empty_ls_out &&
  echo "{\"RemoteServices\":[]}" > exp_ls_out &&
  test_cmp exp_ls_out empty_ls_out
'

# add valid and invalid services
test_expect_success "creating test user on remote pinning service" '
  echo CI host IP address ${TEST_PIN_SVC} &&
  ipfs pin remote service add test_pin_svc ${TEST_PIN_SVC} ${TEST_PIN_SVC_KEY} &&
  ipfs pin remote service add test.dots.in.name ${TEST_PIN_SVC} ${TEST_PIN_SVC_KEY} &&
  ipfs pin remote service add test_invalid_key_svc ${TEST_PIN_SVC} fake_api_key &&
  ipfs pin remote service add test_invalid_url_path_svc ${TEST_PIN_SVC}/invalid-path fake_api_key &&
  ipfs pin remote service add test_invalid_url_dns_svc https://invalid-service.example.com fake_api_key &&
  ipfs pin remote service add test_pin_mfs_svc ${TEST_PIN_SVC} ${TEST_PIN_SVC_KEY}
'

# add a service with a invalid endpoint
test_expect_success "adding remote service with invalid endpoint" '
  test_expect_code 1 ipfs pin remote service add test_endpoint_no_protocol invalid-service.example.com fake_api_key &&
  test_expect_code 1 ipfs pin remote service add test_endpoint_bad_protocol xyz://invalid-service.example.com fake_api_key
'

test_expect_success "test 'ipfs pin remote service ls'" '
  ipfs pin remote service ls | tee ls_out &&
  grep -q test_pin_svc ls_out &&
  grep -q test_invalid_key_svc ls_out &&
  grep -q test_invalid_url_path_svc ls_out &&
  grep -q test_invalid_url_dns_svc ls_out
'

test_expect_success "test enabling mfs pinning" '
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.RepinInterval \"10s\" &&
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.PinName \"mfs_test_pin\" &&
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.Enable true &&
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.RepinInterval > repin_interval &&
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.PinName > pin_name &&
  ipfs config --json Pinning.RemoteServices.test_pin_mfs_svc.Policies.MFS.Enable > enable &&
  echo 10s > expected_repin_interval &&
  echo mfs_test_pin > expected_pin_name &&
  echo true > expected_enable &&
  test_cmp repin_interval expected_repin_interval &&
  test_cmp pin_name expected_pin_name &&
  test_cmp enable expected_enable
'

# expect PIN to be created
test_expect_success "verify MFS root is being pinned" '
  ipfs files cp /ipfs/bafkqaaa /mfs-pinning-test-$(date +%s.%N) &&
  ipfs files flush &&
  sleep 31 &&
  ipfs files stat / --enc=json | jq -r .Hash > mfs_cid &&
  ipfs pin remote ls --service=test_pin_mfs_svc --name=mfs_test_pin --status=queued,pinning,pinned,failed --enc=json | tee ls_out | jq -r .Cid > pin_cid &&
  cat mfs_cid ls_out &&
  test_cmp mfs_cid pin_cid
'

# expect existing PIN to be replaced
test_expect_success "verify MFS root is being repinned on CID change" '
  ipfs files cp /ipfs/bafkqaaa /mfs-pinning-repin-test-$(date +%s.%N) &&
  ipfs files flush &&
  sleep 31 &&
  ipfs files stat / --enc=json | jq -r .Hash > mfs_cid &&
  ipfs pin remote ls --service=test_pin_mfs_svc --name=mfs_test_pin --status=queued,pinning,pinned,failed --enc=json | tee ls_out | jq -r .Cid > pin_cid &&
  cat mfs_cid ls_out &&
  test_cmp mfs_cid pin_cid
'

# SECURITY of access tokens in API.Key fields:
# Pinning.RemoteServices includes API.Key, and we give it the same treatment
# as Identity.PrivKey to prevent exposing it on the network

test_expect_success "'ipfs config Pinning' fails" '
  test_expect_code 1 ipfs config Pinning 2>&1 > config_out
'
test_expect_success "output does not include API.Key" '
  test_expect_code 1 grep -q Key config_out
'

# dot notation

test_expect_success "'ipfs config Pinning.RemoteServices.test_pin_svc.API.Key' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices.test_pin_svc.API.Key 2> config_out
'

test_expect_success "output includes meaningful error" '
  echo "Error: cannot show or change pinning services credentials" > config_exp &&
  test_cmp config_exp config_out
'

test_expect_success "'ipfs config Pinning.RemoteServices.test_pin_svc' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices.test_pin_svc 2> config_out
'
test_expect_success "output includes meaningful error" '
  test_cmp config_exp config_out
'

# json map key notation

test_expect_success "'ipfs config Pinning.RemoteServices[\"test.dots.in.name\"]' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices[\"test.dots.in.name\"] 2> config_out
'
test_expect_success "output includes meaningful error" '
  test_cmp config_exp config_out
'

test_expect_success "'ipfs config Pinning.RemoteServices[\"test.dots.in.name\"].API.Key' fails" '
  test_expect_code 1 ipfs config Pinning.RemoteServices[\"test.dots.in.name\"].API.Key 2> config_out
'

test_expect_success "output includes meaningful error" '
  test_cmp config_exp config_out
'

# config show

test_expect_success "'ipfs config show' does not include Pinning.RemoteServices[*].API.Key" '
  ipfs config show | tee show_config | jq -r .Pinning.RemoteServices > remote_services &&
  test_expect_code 1 grep \"Key\" remote_services &&
  test_expect_code 1 grep fake_api_key show_config &&
  test_expect_code 1 grep "$TEST_PIN_SVC_KEY" show_config
'

test_expect_success "'ipfs config replace' injects Pinning.RemoteServices[*].API.Key back" '
  test_expect_code 1 grep fake_api_key show_config &&
  test_expect_code 1 grep "$TEST_PIN_SVC_KEY" show_config &&
  ipfs config replace show_config &&
  test_expect_code 0 grep fake_api_key "$IPFS_PATH/config" &&
  test_expect_code 0 grep "$TEST_PIN_SVC_KEY" "$IPFS_PATH/config"
'

# note: we remove Identity.PrivKey to ensure error is triggered by Pinning.RemoteServices
test_expect_success "'ipfs config replace' with Pinning.RemoteServices[*].API.Key errors out" '
  jq -M "del(.Identity.PrivKey)" "$IPFS_PATH/config" | jq ".Pinning += { RemoteServices: {\"myservice\": {\"API\": {\"Endpoint\": \"https://example.com/psa\", \"Key\": \"mysecret\"}}}}" > new_config &&
  test_expect_code 1 ipfs config replace - < new_config 2> replace_out
'
test_expect_success "output includes meaningful error" "
  echo \"Error: cannot add or remove remote pinning services with 'config replace'\" > replace_expected &&
  test_cmp replace_out replace_expected
"

# /SECURITY

test_expect_success "pin remote service ls --stat' returns numbers for a valid service" '
  ipfs pin remote service ls --stat | grep -E "^test_pin_svc.+[0-9]+/[0-9]+/[0-9]+/[0-9]+$"
'

test_expect_success "pin remote service ls --enc=json --stat' returns valid status" "
  ipfs pin remote service ls --stat --enc=json | jq --raw-output '.RemoteServices[] | select(.Service == \"test_pin_svc\") | .Stat.Status' | tee stat_out &&
  echo valid > stat_expected &&
  test_cmp stat_out stat_expected
"

test_expect_success "pin remote service ls --stat' returns invalid status for invalid service" '
  ipfs pin remote service ls --stat | grep -E "^test_invalid_url_path_svc.+invalid$"
'

test_expect_success "pin remote service ls --enc=json --stat' returns invalid status" "
  ipfs pin remote service ls --stat --enc=json | jq --raw-output '.RemoteServices[] | select(.Service == \"test_invalid_url_path_svc\") | .Stat.Status' | tee stat_out &&
  echo invalid > stat_expected &&
  test_cmp stat_out stat_expected
"

test_expect_success "pin remote service ls --enc=json' (without --stat) returns no Stat object" "
  ipfs pin remote service ls --enc=json | jq --raw-output '.RemoteServices[] | select(.Service == \"test_invalid_url_path_svc\") | .Stat' | tee stat_out &&
  echo null > stat_expected &&
  test_cmp stat_out stat_expected
"

test_expect_success "check connection to the test pinning service" '
  ipfs pin remote ls --service=test_pin_svc --enc=json
'

test_expect_success "unauthorized pinning service calls fail" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_key_svc
'

test_expect_success "misconfigured pinning service calls fail (wrong path)" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_url_path_svc
'

test_expect_success "misconfigured pinning service calls fail (dns error)" '
  test_expect_code 1 ipfs pin remote ls --service=test_invalid_url_dns_svc
'

# pin remote service rm

test_expect_success "remove pinning service" '
  ipfs pin remote service rm test_invalid_key_svc &&
  ipfs pin remote service rm test_invalid_url_path_svc &&
  ipfs pin remote service rm test_invalid_url_dns_svc
'

test_expect_success "verify pinning service removal works" '
  ipfs pin remote service ls | tee ls_out &&
  test_expect_code 1 grep test_invalid_key_svc ls_out &&
  test_expect_code 1 grep test_invalid_url_path_svc ls_out &&
  test_expect_code 1 grep test_invalid_url_dns_svc ls_out
'

# pin remote add

# we leverage the fact that inlined CID can be pinned instantly on the remote service
# (https://github.com/ipfs-shipyard/rb-pinning-service-api/issues/8)
# below test ensures that assumption is correct (before we proceed to actual tests)
test_expect_success "verify that default add (implicit --background=false) works with data inlined in CID" '
  ipfs pin remote add --service=test_pin_svc --name=inlined_null bafkqaaa &&
  ipfs pin remote ls --service=test_pin_svc --enc=json --name=inlined_null --status=pinned | jq --raw-output .Status | tee ls_out &&
  grep -q "pinned" ls_out
'

test_remote_pins() {
  BASE=$1
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  # note: HAS_MISSING is not inlined nor imported to IPFS on purpose, to reliably test 'queued' state
  test_expect_success "create some hashes using base $BASE" '
    export HASH_A=$(echo -n "A @ $(date +%s.%N)" | ipfs add $BASE_ARGS -q --inline --inline-limit 1000 --pin=false) &&
    export HASH_B=$(echo -n "B @ $(date +%s.%N)" | ipfs add $BASE_ARGS -q --inline --inline-limit 1000 --pin=false) &&
    export HASH_C=$(echo -n "C @ $(date +%s.%N)" | ipfs add $BASE_ARGS -q --inline --inline-limit 1000 --pin=false) &&
    export HASH_MISSING=$(echo "MISSING FROM IPFS @ $(date +%s.%N)" | ipfs add $BASE_ARGS -q --only-hash) &&
    echo "A: $HASH_A" &&
    echo "B: $HASH_B" &&
    echo "C: $HASH_C" &&
    echo "M: $HASH_MISSING"
  '

  test_expect_success "'ipfs pin remote add --background=true'" '
    ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_a $HASH_A
  '

  test_expect_success "verify background add worked (instantly pinned variant)" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | tee ls_out &&
    test_expect_code 0 grep -q name_a ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out
  '

  test_expect_success "'ipfs pin remote add --background=true' with CID that is not available" '
    test_expect_code 0 ipfs pin remote add --background=true --service=test_pin_svc --enc=json $BASE_ARGS --name=name_m $HASH_MISSING
  '

  test_expect_success "verify background add worked (queued variant)" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_m --status=queued,pinning | tee ls_out &&
    test_expect_code 0 grep -q name_m ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote add --background=false'" '
    test_expect_code 0 ipfs pin remote add --background=false --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_B
  '

  test_expect_success "verify foreground add worked" '
    ipfs pin remote ls --service=test_pin_svc --enc=json $ID_B | tee ls_out &&
    test_expect_code 0 grep -q name_b ls_out &&
    test_expect_code 0 grep -q pinned ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by multiple statuses" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning,pinned,failed | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by CID" '
    ipfs pin remote ls --service=test_pin_svc --enc=json  --cid=$HASH_B | tee ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out
  '

  test_expect_success "'ipfs pin remote ls' for existing pins by name" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_a | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out
  '

  test_expect_success "'ipfs pin remote ls' for ongoing pins by status" '
    ipfs pin remote ls --service=test_pin_svc --status=queued,pinning | tee ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out
  '

  # --force is required only when more than a single match is found,
  # so we add second pin with the same name (but different CID) to simulate that scenario
  test_expect_success "'ipfs pin remote rm --name' fails without --force when matching multiple pins" '
    test_expect_code 0 ipfs pin remote add --service=test_pin_svc --enc=json $BASE_ARGS --name=name_b $HASH_C &&
    test_expect_code 1 ipfs pin remote rm --service=test_pin_svc --name=name_b 2> rm_out &&
    echo "Error: multiple remote pins are matching this query, add --force to confirm the bulk removal" > rm_expected &&
    test_cmp rm_out rm_expected
  '

  test_expect_success "'ipfs pin remote rm --name' without --force did not remove matching pins" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_b | jq --raw-output .Cid | tee ls_out &&
    test_expect_code 0 grep -q $HASH_B ls_out &&
    test_expect_code 0 grep -q $HASH_C ls_out
  '

  test_expect_success "'ipfs pin remote rm --name' with --force removes all matching pins" '
    test_expect_code 0 ipfs pin remote rm --service=test_pin_svc --name=name_b --force &&
    ipfs pin remote ls --service=test_pin_svc --enc=json --name=name_b | jq --raw-output .Cid | tee ls_out &&
    test_expect_code 1 grep -q $HASH_B ls_out &&
    test_expect_code 1 grep -q $HASH_C ls_out
  '

  test_expect_success "'ipfs pin remote rm --force' removes all pinned items" '
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning,pinned,failed | jq --raw-output .Cid | tee ls_out &&
    test_expect_code 0 grep -q $HASH_A ls_out &&
    test_expect_code 0 grep -q $HASH_MISSING ls_out &&
    ipfs pin remote rm --service=test_pin_svc --status=queued,pinning,pinned,failed --force &&
    ipfs pin remote ls --service=test_pin_svc --enc=json --status=queued,pinning,pinned,failed | jq --raw-output .Cid | tee ls_out &&
    test_expect_code 1 grep -q $HASH_A ls_out &&
    test_expect_code 1 grep -q $HASH_MISSING ls_out
  '

}

test_remote_pins ""

test_kill_ipfs_daemon
test_done

# vim: ts=2 sw=2 sts=2 et:
