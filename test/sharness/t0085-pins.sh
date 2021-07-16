#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ipfs pinning operations"

. lib/test-lib.sh


test_pins() {
  PIN_ARGS="$1"
  LS_ARGS="$2"
  BASE=$3
  if [ -n "$BASE" ]; then
    BASE_ARGS="--cid-base=$BASE"
  fi

  test_expect_success "create some hashes $BASE" '
    HASH_A=$(echo "A" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_B=$(echo "B" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_C=$(echo "C" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_D=$(echo "D" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_E=$(echo "E" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_F=$(echo "F" | ipfs add $BASE_ARGS -q --pin=false) &&
    HASH_G=$(echo "G" | ipfs add $BASE_ARGS -q --pin=false)
  '

  test_expect_success "put all those hashes in a file" '
    echo $HASH_A > hashes &&
    echo $HASH_B >> hashes &&
    echo $HASH_C >> hashes &&
    echo $HASH_D >> hashes &&
    echo $HASH_E >> hashes &&
    echo $HASH_F >> hashes &&
    echo $HASH_G >> hashes
  '

  if [ -n "$BASE" ]; then
    test_expect_success "make sure hashes are in $BASE" '
      cat hashes | xargs cid-fmt %b | sort -u > actual
      echo base32 > expected
      test_cmp expected actual
    '
  fi

  test_expect_success "'ipfs pin add $PIN_ARGS' via stdin" '
    cat hashes | ipfs pin add $PIN_ARGS $BASE_ARGS | tee actual
  '

  test_expect_success "'ipfs pin add $PIN_ARGS' output looks good" '
    sed -e "s/^/pinned /; s/$/ recursively/" hashes > expected &&
    test_cmp expected actual
  '

  test_expect_success "see if verify works" '
    ipfs pin verify
  '

  test_expect_success "see if verify --verbose $BASE_ARGS works" '
    ipfs pin verify --verbose $BASE_ARGS > verify_out &&
    test $(cat verify_out | wc -l) -ge 7 &&
    test_should_contain "$HASH_A ok" verify_out &&
    test_should_contain "$HASH_B ok" verify_out &&
    test_should_contain "$HASH_C ok" verify_out &&
    test_should_contain "$HASH_D ok" verify_out &&
    test_should_contain "$HASH_E ok" verify_out &&
    test_should_contain "$HASH_F ok" verify_out &&
    test_should_contain "$HASH_G ok" verify_out
  '

  test_expect_success "ipfs pin ls $LS_ARGS $BASE_ARGS works" '
    ipfs pin ls $LS_ARGS $BASE_ARGS > ls_out &&
    test_should_contain "$HASH_A" ls_out &&
    test_should_contain "$HASH_B" ls_out &&
    test_should_contain "$HASH_C" ls_out &&
    test_should_contain "$HASH_D" ls_out &&
    test_should_contain "$HASH_E" ls_out &&
    test_should_contain "$HASH_F" ls_out &&
    test_should_contain "$HASH_G" ls_out
  '

  test_expect_success "test pin ls $LS_ARGS $BASE_ARGS hash" '
    echo $HASH_B | test_must_fail grep /ipfs && # just to be sure
    ipfs pin ls $LS_ARGS $BASE_ARGS $HASH_B > ls_hash_out &&
    echo "$HASH_B recursive" > ls_hash_exp &&
    test_cmp ls_hash_exp ls_hash_out
  '

  test_expect_success "unpin those hashes" '
    cat hashes | ipfs pin rm
  '

  test_expect_success "test pin update" '
    ipfs pin add "$HASH_A" &&
    ipfs pin ls $LS_ARGS $BASE_ARGS | tee before_update &&
    test_should_contain "$HASH_A" before_update &&
    test_must_fail grep -q "$HASH_B" before_update &&
    ipfs pin update --unpin=true "$HASH_A" "$HASH_B" &&
    ipfs pin ls $LS_ARGS $BASE_ARGS > after_update &&
    test_must_fail grep -q "$HASH_A" after_update &&
    test_should_contain "$HASH_B" after_update &&
    ipfs pin update --unpin=true "$HASH_B" "$HASH_B" &&
    ipfs pin ls $LS_ARGS $BASE_ARGS > after_idempotent_update &&
    test_should_contain "$HASH_B" after_idempotent_update &&
    ipfs pin rm "$HASH_B"
  '
}

RANDOM_HASH=Qme8uX5n9hn15pw9p6WcVKoziyyC9LXv4LEgvsmKMULjnV

test_pins_error_reporting() {
  PIN_ARGS=$1

  test_expect_success "'ipfs pin add $PIN_ARGS' on non-existent hash should fail" '
    test_must_fail ipfs pin add $PIN_ARGS $RANDOM_HASH 2> err &&
    grep -q "not found" err
  '
}

test_pin_dag_init() {
  PIN_ARGS=$1

  test_expect_success "'ipfs add $PIN_ARGS --pin=false' 1MB file" '
    random 1048576 56 > afile &&
    HASH=`ipfs add $PIN_ARGS --pin=false -q afile`
  '
}

test_pin_dag() {
  test_pin_dag_init $1

  test_expect_success "'ipfs pin add --progress' file" '
    ipfs pin add --recursive=true $HASH
  '

  test_expect_success "'ipfs pin rm' file" '
    ipfs pin rm $HASH
  '

  test_expect_success "remove part of the dag" '
    PART=`ipfs refs $HASH | head -1` &&
    ipfs block rm $PART
  '

  test_expect_success "pin file, should fail" '
    test_must_fail ipfs pin add --recursive=true $HASH 2> err &&
    cat err &&
    grep -q "not found" err
  '
}

test_pin_progress() {
  test_pin_dag_init

  test_expect_success "'ipfs pin add --progress' file" '
    ipfs pin add --progress $HASH 2> err
  '

  test_expect_success "pin progress reported correctly" '
    cat err
    grep -q " 5 nodes" err
  '
}

test_init_ipfs

test_pins '' '' ''
test_pins --progress '' ''
test_pins --progress --stream ''
test_pins '' '' base32
test_pins '' --stream base32

test_pins_error_reporting
test_pins_error_reporting --progress

test_pin_dag
test_pin_dag --raw-leaves

test_pin_progress

test_launch_ipfs_daemon_without_network

test_pins '' '' ''
test_pins --progress '' ''
test_pins --progress --stream ''
test_pins '' '' base32
test_pins '' --stream base32

test_pins_error_reporting
test_pins_error_reporting --progress

test_pin_dag
test_pin_dag --raw-leaves

test_pin_progress

test_kill_ipfs_daemon

test_done
