#!/usr/bin/env bash
#
# Copyright (c) 2014 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs repo operations"

. lib/test-lib.sh

test_name_with_self() {
        SELF_ALG=$1

        test_expect_success "ipfs init (variant self $SELF_ALG)" '
        export IPFS_PATH="$(pwd)/.ipfs" &&
        case $SELF_ALG in
        default)
                ipfs init --profile=test > /dev/null
                ;;
        rsa)
                ipfs init --profile=test -a=rsa > /dev/null
                ;;
        ed25519)
                ipfs init --profile=test -a=ed25519 > /dev/null
                ;;
        esac &&
        export PEERID=`ipfs key list --ipns-base=base36 -l | grep self | cut -d " " -f1` &&
        test_check_peerid "${PEERID}"
        '

        # test publishing a hash

        test_expect_success "'ipfs name publish --allow-offline' succeeds" '
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

        # test publishing with -Q option

        test_expect_success "'ipfs name publish --quieter' succeeds" '
        ipfs name publish --allow-offline -Q "/ipfs/$HASH_WELCOME_DOCS" >publish_out
        '

        test_expect_success "publish --quieter output looks good" '
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

        # now test with a path

        test_expect_success "'ipfs name publish --allow-offline' succeeds" '
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
        echo ipfs name publish --allow-offline "${PEERID}" "/ipfs/$HASH_WELCOME_DOCS" &&
        ipfs name publish --allow-offline "${PEERID}" "/ipfs/$HASH_WELCOME_DOCS" >actual_node_id_publish
        '

        test_expect_failure "publish with our explicit node ID looks good" '
        echo "Published to ${PEERID}: /ipfs/$HASH_WELCOME_DOCS" >expected_node_id_publish &&
        test_cmp expected_node_id_publish actual_node_id_publish
        '

        # test publishing with B36CID and B58MH resolve to the same B36CID

        test_expect_success "verify self key output" '
        B58MH_ID=`ipfs key list --ipns-base=b58mh -l | grep self | cut -d " " -f1` &&
        B36CID_ID=`ipfs key list --ipns-base=base36 -l | grep self | cut -d " " -f1` &&
        test_check_peerid "${B58MH_ID}" &&
        test_check_peerid "${B36CID_ID}"
        '

        test_expect_success "'ipfs name publish --allow-offline --key=<peer-id> <hash>' succeeds" '
        ipfs name publish --allow-offline  --key=${B58MH_ID} "/ipfs/$HASH_WELCOME_DOCS" >b58mh_published_id_base36 &&
        ipfs name publish --allow-offline  --key=${B36CID_ID} "/ipfs/$HASH_WELCOME_DOCS" >base36_published_id_base36 &&
        ipfs name publish --allow-offline  --key=${B58MH_ID} --ipns-base=b58mh "/ipfs/$HASH_WELCOME_DOCS" >b58mh_published_id_b58mh &&
        ipfs name publish --allow-offline  --key=${B36CID_ID} --ipns-base=b58mh "/ipfs/$HASH_WELCOME_DOCS" >base36_published_id_b58mh
        '

        test_expect_success "publish an explicit node ID as two key in B58MH and B36CID, name looks good" '
        echo "Published to ${B36CID_ID}: /ipfs/$HASH_WELCOME_DOCS" >expected_published_id_base36 &&
        echo "Published to ${B58MH_ID}: /ipfs/$HASH_WELCOME_DOCS" >expected_published_id_b58mh &&
        test_cmp expected_published_id_base36 b58mh_published_id_base36 &&
        test_cmp expected_published_id_base36 base36_published_id_base36 &&
        test_cmp expected_published_id_b58mh b58mh_published_id_b58mh &&
        test_cmp expected_published_id_b58mh base36_published_id_b58mh
        '

        test_expect_success "'ipfs name resolve' succeeds" '
        ipfs name resolve "$B36CID_ID" >output
        '

        test_expect_success "resolve output looks good" '
        printf "/ipfs/%s\n" "$HASH_WELCOME_DOCS" >expected2 &&
        test_cmp expected2 output
        '

        # test IPNS + IPLD

        test_expect_success "'ipfs dag put' succeeds" '
        HELLO_HASH="$(echo "\"hello world\"" | ipfs dag put)" &&
        OBJECT_HASH="$(echo "{\"thing\": {\"/\": \"${HELLO_HASH}\" }}" | ipfs dag put)"
        '
        test_expect_success "'ipfs name publish --allow-offline /ipld/...' succeeds" '
        test_check_peerid "${PEERID}" &&
        ipfs name publish --allow-offline "/ipld/$OBJECT_HASH/thing" >publish_out
        '
        test_expect_success "publish a path looks good" '
        echo "Published to ${PEERID}: /ipld/$OBJECT_HASH/thing" >expected3 &&
        test_cmp expected3 publish_out
        '
        test_expect_success "'ipfs name resolve' succeeds" '
        ipfs name resolve "$PEERID" >output
        '
        test_expect_success "resolve output looks good (IPNS + IPLD)" '
        printf "/ipld/%s/thing\n" "$OBJECT_HASH" >expected4 &&
        test_cmp expected4 output
        '

        # test publishing nothing

        test_expect_success "'ipfs name publish' fails" '
        printf '' | test_expect_code 1 ipfs name publish --allow-offline  >publish_out 2>&1
        '

        test_expect_success "publish output has the correct error" '
        grep "argument \"ipfs-path\" is required" publish_out
        '

        test_expect_success "'ipfs name publish' fails" '
        printf '' | test_expect_code 1 ipfs name publish -Q --allow-offline  >publish_out 2>&1
        '

        test_expect_success "publish output has the correct error" '
        grep "argument \"ipfs-path\" is required" publish_out
        '

        test_expect_success "'ipfs name publish --help' succeeds" '
        ipfs name publish --help
        '

        # test offline resolve

        test_expect_success "'ipfs name resolve --offline' succeeds" '
        ipfs name resolve --offline "$PEERID" >output
        '
        test_expect_success "resolve output looks good (offline resolve)" '
        printf "/ipld/%s/thing\n" "$OBJECT_HASH" >expected4 &&
        test_cmp expected4 output
        '

        test_expect_success "'ipfs name resolve --offline -n' succeeds" '
        ipfs name resolve --offline -n "$PEERID" >output
        '
        test_expect_success "resolve output looks good (offline resolve, -n)" '
        printf "/ipld/%s/thing\n" "$OBJECT_HASH" >expected4 &&
        test_cmp expected4 output
        '

        test_launch_ipfs_daemon

        test_expect_success "'ipfs name resolve --offline' succeeds" '
        ipfs name resolve --offline "$PEERID" >output
        '
        test_expect_success "resolve output looks good (with daemon)" '
        printf "/ipld/%s/thing\n" "$OBJECT_HASH" >expected4 &&
        test_cmp expected4 output
        '

        test_expect_success "'ipfs name resolve --offline -n' succeeds" '
        ipfs name resolve --offline -n "$PEERID" >output
        '
        test_expect_success "resolve output looks good (with daemon, -n)" '
        printf "/ipld/%s/thing\n" "$OBJECT_HASH" >expected4 &&
        test_cmp expected4 output
        '

        test_expect_success "empty request to name publish doesn't panic and returns error" '
        curl -X POST "http://$API_ADDR/api/v0/name/publish" > curl_out || true &&
        grep "argument \"ipfs-path\" is required" curl_out
        '

        test_kill_ipfs_daemon

        # Test daemon in offline mode
        test_launch_ipfs_daemon_without_network

        test_expect_success "'ipfs name publish' fails offline mode" '
        test_expect_code 1 ipfs name publish "/ipfs/$HASH_WELCOME_DOCS"
        '

        test_kill_ipfs_daemon

        test_expect_success "clean up ipfs dir" '
        rm -rf "$IPFS_PATH"
        '
}
test_name_with_self 'default'
test_name_with_self 'rsa'
test_name_with_self 'ed25519'

test_name_with_key() {
        GEN_ALG=$1

        test_expect_success "ipfs init (key variant $GEN_ALG)" '
        export IPFS_PATH="$(pwd)/.ipfs" &&
        ipfs init --profile=test > /dev/null
        '

        test_expect_success "'prepare keys" '
        case $GEN_ALG in
        rsa)
                export KEY=`ipfs key gen --ipns-base=b58mh --type=rsa --size=2048 key` &&
                export KEY_B36CID=`ipfs key list --ipns-base=base36 -l | grep key | cut -d " " -f1`
                ;;
        ed25519_b58)
                export KEY=`ipfs key gen --ipns-base=b58mh --type=ed25519 key`
                export KEY_B36CID=`ipfs key list --ipns-base=base36 -l | grep key | cut -d " " -f1`
                ;;
        ed25519_b36)
                export KEY=`ipfs key gen --ipns-base=base36 --type=ed25519 key`
                export KEY_B36CID=$KEY
                ;;
        esac &&
        test_check_peerid "${KEY}"
        '

        # publish with an explicit node ID as key name

        test_expect_success "'ipfs name publish --allow-offline --key=<peer-id> <hash>' succeeds" '
        ipfs name publish --allow-offline  --key=${KEY} "/ipfs/$HASH_WELCOME_DOCS" >actual_node_id_publish
        '

        test_expect_success "publish an explicit node ID as key name looks good" '
        echo "Published to ${KEY_B36CID}: /ipfs/$HASH_WELCOME_DOCS" >expected_node_id_publish &&
        test_cmp expected_node_id_publish actual_node_id_publish
        '

        # cleanup
        test_expect_success "clean up ipfs dir" '
        rm -rf "$IPFS_PATH"
        '
}
test_name_with_key 'rsa'
test_name_with_key 'ed25519_b58'
test_name_with_key 'ed25519_b36'

test_done
