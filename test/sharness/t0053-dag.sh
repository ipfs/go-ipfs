#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test dag command"

. lib/test-lib.sh

test_init_ipfs

test_expect_success "make a few test files" '
  echo "foo" > file1 &&
  echo "bar" > file2 &&
  echo "baz" > file3 &&
  echo "qux" > file4 &&
  HASH1=$(ipfs add --pin=false -q file1) &&
  HASH2=$(ipfs add --pin=false -q file2) &&
  HASH3=$(ipfs add --pin=false -q file3) &&
  HASH4=$(ipfs add --pin=false -q file4)
'

test_expect_success "make an ipld object in json" '
  printf "{\"cats\":[{\"/\":\"%s\"},{\"water\":{\"/\":\"%s\"}}],\"hello\":\"world\",\"magic\":{\"/\":\"%s\"},\"subNode\":{\"beep\":[0,\"bop\"],\"dict\":\"ionary\"}}" $HASH1 $HASH2 $HASH3 > ipld_object
'

test_expect_success "make the same ipld object in dag-cbor, dag-json and dag-pb" '
  echo "omREYXRhT6JkRGF0YWF4ZUxpbmtzgGVMaW5rc4A=" | base64 -d > ipld_object_dagcbor
  echo "Cg+iZERhdGFheGVMaW5rc4A=" | base64 -d > ipld_object_dagpb
  echo "{\"Data\":{\"/\":{\"bytes\":\"omREYXRhYXhlTGlua3OA\"}},\"Links\":[]}" > ipld_object_dagjson
'

test_dag_cmd() {
  test_expect_success "can add an ipld object using dag-json to dag-cbor" '
    IPLDHASH=$(cat ipld_object | ipfs dag put --input-enc=dag-json -f dag-cbor)
  '

  test_expect_success "CID looks correct" '
    EXPHASH="bafyreihim5n3etxp5om74fdpriqtys5sdmgbd5ix2oil7u2g2qfdpdlgie"
    test $EXPHASH = $IPLDHASH
  '

  test_expect_success "can add an ipld object using defaults" '
    IPLDHASH=$(cat ipld_object | ipfs dag put)
  '

  test_expect_success "CID looks correct" '
    EXPHASH="bafyreihim5n3etxp5om74fdpriqtys5sdmgbd5ix2oil7u2g2qfdpdlgie"
    test $EXPHASH = $IPLDHASH
  '

  test_expect_success "can add an ipld object using protobuf and --cid=base=base32" '
    IPLDHASHb32=$(cat ipld_object | ipfs dag put --input-enc=dag-json -f dag-cbor --cid-base=base32)
  '

  test_expect_success "output looks correct (does not upgrade to CIDv1)" '
    test $EXPHASH = $IPLDHASHb32
  '

  test_expect_success "can add an ipld object using --cid-base=base32" '
    IPLDHASHb32=$(cat ipld_object | ipfs dag put --cid-base=base32)
  '

  test_expect_success "output looks correct" '
    test $(ipfs cid base32 $EXPHASH) = $IPLDHASHb32
  '

  test_expect_success "can add an ipld object using dag-cbor to dag-pb" '
    IPLDPBHASH=$(cat ipld_object_dagcbor | ipfs dag put --input-enc=dag-cbor -f dag-pb)
  '

  test_expect_success "dag-pb CID looks correct" '
    EXPHASH="bafybeicbvnvha5b25w77plipz3v5axqydbhxb2s2ojglexotowwieoecvy"
    test $EXPHASH = $IPLDPBHASH
  '

  test_expect_success "can add an ipld object using dag-cbor to dag-cbor" '
    IPLDCBORHASH=$(cat ipld_object_dagcbor | ipfs dag put --input-enc=dag-cbor -f dag-cbor)
  '

  test_expect_success "dag-cbor CID looks correct" '
    EXPHASH="bafyreibuwyi6b7oruz64krsakf5a4zmttu6dqksu6bpkpy3sjrm6a73o6a"
    test $EXPHASH = $IPLDCBORHASH
  '

  test_expect_success "can add an ipld object using dag-cbor to dag-json" '
    IPLDJSONHASH=$(cat ipld_object_dagcbor | ipfs dag put --input-enc=dag-cbor -f dag-json)
  '

  test_expect_success "dag-cbor CID looks correct" '
    EXPHASH="baguqeerarvo2dwtckziergct46ijehc4yc7o3y7rwsiomwf65frbhqtgc4ta"
    test $EXPHASH = $IPLDJSONHASH
  '

  test_expect_success "various path traversals work" '
    ipfs cat $IPLDHASH/cats/0 > out1 &&
    ipfs cat $IPLDHASH/cats/1/water > out2 &&
    ipfs cat $IPLDHASH/magic > out3
  '

  test_expect_success "outputs look correct" '
    test_cmp file1 out1 &&
    test_cmp file2 out2 &&
    test_cmp file3 out3
  '

  test_expect_success "resolving sub-objects works" '
    ipfs dag get $IPLDHASH/hello > sub1 &&
    ipfs dag get $IPLDHASH/subNode > sub2 &&
    ipfs dag get $IPLDHASH/subNode/beep > sub3 &&
    ipfs dag get $IPLDHASH/subNode/beep/0 > sub4 &&
    ipfs dag get $IPLDHASH/subNode/beep/1 > sub5
  '

  test_expect_success "sub-objects look right" '
    echo -n "\"world\"" > sub1_exp &&
    test_cmp sub1_exp sub1 &&
    echo -n "{\"beep\":[0,\"bop\"],\"dict\":\"ionary\"}" > sub2_exp &&
    test_cmp sub2_exp sub2 &&
    echo -n "[0,\"bop\"]" > sub3_exp &&
    test_cmp sub3_exp sub3 &&
    echo -n "0" > sub4_exp &&
    test_cmp sub4_exp sub4 &&
    echo -n "\"bop\"" > sub5_exp &&
    test_cmp sub5_exp sub5
  '

  test_expect_success "can pin ipld object" '
    ipfs pin add $IPLDHASH
  '

  test_expect_success "can pin dag-pb object" '
    ipfs pin add $IPLDPBHASH
  '

  test_expect_success "can pin dag-cbor object" '
    ipfs pin add $IPLDCBORHASH
  '

  test_expect_success "can pin dag-json object" '
    ipfs pin add $IPLDJSONHASH
  '

  test_expect_success "after gc, objects still accessible" '
    ipfs repo gc > /dev/null &&
    ipfs refs -r --timeout=2s $EXPHASH > /dev/null
  '

  test_expect_success "can get object" '
    ipfs dag get $IPLDHASH > ipld_obj_out
  '

  test_expect_success "object links look right" '
    grep "{\"/\":\"" ipld_obj_out > /dev/null
  '

  test_expect_success "retrieved object hashes back correctly" '
    IPLDHASH2=$(cat ipld_obj_out | ipfs dag put --input-enc=dag-json -f dag-cbor) &&
    test "$IPLDHASH" = "$IPLDHASH2"
  '

  test_expect_success "add a normal file" '
    HASH=$(echo "foobar" | ipfs add -q)
  '

  test_expect_success "can view protobuf object with dag get" '
    ipfs dag get $HASH > dag_get_pb_out
  '

  test_expect_success "output looks correct" '
    echo -n "{\"Data\":{\"/\":{\"bytes\":\"CAISB2Zvb2JhcgoYBw==\"}},\"Links\":[]}" > dag_get_pb_exp &&
    test_cmp dag_get_pb_exp dag_get_pb_out
  '

  test_expect_success "can call dag get with a path" '
    ipfs dag get $IPLDHASH/cats/0 > cat_out
  '

  test_expect_success "output looks correct" '
    echo -n "{\"Data\":{\"/\":{\"bytes\":\"CAISBGZvbwoYBA==\"}},\"Links\":[]}" > cat_exp &&
    test_cmp cat_exp cat_out
  '

  test_expect_success "non-canonical cbor input is normalized" '
    HASH=$(cat ../t0053-dag-data/non-canon.cbor | ipfs dag put --format=dag-cbor --input-enc=dag-cbor) &&
    test $HASH = "bafyreicfngo3hexoxbbwsm64apbydw5fyeruyrfwiyxlj3f4wuzdyztd4m" ||
    test_fsh echo $HASH
  '

  test_expect_success "cbor input can be fetched" '
    EXPARR=$(ipfs dag get $HASH/arr)
    test $EXPARR = "[]"
  '

  test_expect_success "add an ipld with pin" '
    PINHASH=$(printf {\"foo\":\"bar\"} | ipfs dag put --input-enc=dag-json --pin=true)
  '

  test_expect_success "after gc, objects still accessible" '
    ipfs repo gc > /dev/null &&
    ipfs refs -r --timeout=2s $PINHASH > /dev/null
  '

  test_expect_success "can add an ipld object with sha3 hash" '
    IPLDHASH=$(cat ipld_object | ipfs dag put --hash sha3)
  '

  test_expect_success "output looks correct" '
    EXPHASH="bafyriqdoys7rmgfkjg7jjdjdhaajt66vxjusd2ry6eims63xy5wwjtjgditq4z4f76tmnal5rccb4zg3o4zcud3utxulm7svoapq2du3xmnok"
    test $EXPHASH = $IPLDHASH
  '

  test_expect_success "prepare dag-pb object" '
    echo foo > test_file &&
    HASH=$(ipfs add -wq test_file | tail -n1 | ipfs cid base32)
  '

  test_expect_success "dag put with json dag-pb works" '
    ipfs dag get $HASH > pbjson &&
    cat pbjson | ipfs dag put --format=dag-pb --input-enc=dag-json > dag_put_out
  '

  test_expect_success "dag put with dag-pb works output looks good" '
    echo $HASH > dag_put_exp &&
    test_cmp dag_put_exp dag_put_out
  '

  test_expect_success "dag put with raw dag-pb works" '
    ipfs block get $HASH > pbraw &&
    cat pbraw | ipfs dag put --format=dag-pb --input-enc=dag-pb > dag_put_out
  '

  test_expect_success "dag put with dag-pb works output looks good" '
    echo $HASH > dag_put_exp &&
    test_cmp dag_put_exp dag_put_out
  '

  test_expect_success "dag put with raw node works" '
    echo "foo bar" > raw_node_in &&
    HASH=$(ipfs dag put --format=raw --input-enc=raw -- raw_node_in) &&
    ipfs block get "$HASH" > raw_node_out &&
    test_cmp raw_node_in raw_node_out'

  test_expect_success "dag put multiple files" '
    printf {\"foo\":\"bar\"} > a.json &&
    printf {\"foo\":\"baz\"} > b.json &&
    ipfs dag put a.json b.json > dag_put_out
  '

  test_expect_success "dag put multiple files output looks good" '
    echo bafyreiblaotetvwobe7cu2uqvnddr6ew2q3cu75qsoweulzku2egca4dxq > dag_put_exp &&
    echo bafyreibqp7zvp6dvrqhtkbwuzzk7jhtmfmngtiqjajqpm6gtw55o7kqzfi >> dag_put_exp &&

    test_cmp dag_put_exp dag_put_out
  '

  test_expect_success "prepare data for dag resolve" '
    NESTED_HASH=$(echo "{\"data\":123}" | ipfs dag put) &&
    HASH=$(echo "{\"obj\":{\"/\":\"${NESTED_HASH}\"}}" | ipfs dag put)
  '

  test_expect_success "dag resolve some things" '
    ipfs dag resolve $HASH > resolve_hash &&
    ipfs dag resolve ${HASH}/obj > resolve_obj &&
    ipfs dag resolve ${HASH}/obj/data > resolve_data
  '

  test_expect_success "dag resolve output looks good" '
    printf $HASH > resolve_hash_exp &&
    printf $NESTED_HASH > resolve_obj_exp &&
    printf $NESTED_HASH/data > resolve_data_exp &&

    test_cmp resolve_hash_exp resolve_hash &&
    test_cmp resolve_obj_exp resolve_obj &&
    test_cmp resolve_data_exp resolve_data
  '

  test_expect_success "get base32 version of hashes for testing" '
    HASHb32=$(ipfs cid base32 $HASH) &&
    NESTED_HASHb32=$(ipfs cid base32 $NESTED_HASH)
  '

  test_expect_success "dag resolve some things with --cid-base=base32" '
    ipfs dag resolve $HASH --cid-base=base32 > resolve_hash &&
    ipfs dag resolve ${HASH}/obj --cid-base=base32 > resolve_obj &&
    ipfs dag resolve ${HASH}/obj/data --cid-base=base32 > resolve_data
  '

  test_expect_success "dag resolve output looks good with --cid-base=base32" '
    printf $HASHb32 > resolve_hash_exp &&
    printf $NESTED_HASHb32 > resolve_obj_exp &&
    printf $NESTED_HASHb32/data > resolve_data_exp &&

    test_cmp resolve_hash_exp resolve_hash &&
    test_cmp resolve_obj_exp resolve_obj &&
    test_cmp resolve_data_exp resolve_data
  '

  test_expect_success "dag resolve some things with base32 hash" '
    ipfs dag resolve $HASHb32 > resolve_hash &&
    ipfs dag resolve ${HASHb32}/obj  > resolve_obj &&
    ipfs dag resolve ${HASHb32}/obj/data > resolve_data
  '

  test_expect_success "dag resolve output looks good with base32 hash" '
    printf $HASHb32 > resolve_hash_exp &&
    printf $NESTED_HASHb32 > resolve_obj_exp &&
    printf $NESTED_HASHb32/data > resolve_data_exp &&

    test_cmp resolve_hash_exp resolve_hash &&
    test_cmp resolve_obj_exp resolve_obj &&
    test_cmp resolve_data_exp resolve_data
  '

  test_expect_success "dag stat of simple IPLD object" '
    ipfs dag stat $NESTED_HASH > actual_stat_inner_ipld_obj &&
    echo "Size: 8, NumBlocks: 1" > exp_stat_inner_ipld_obj &&
    test_cmp exp_stat_inner_ipld_obj actual_stat_inner_ipld_obj &&
    ipfs dag stat $HASH > actual_stat_ipld_obj &&
    echo "Size: 54, NumBlocks: 2" > exp_stat_ipld_obj &&
    test_cmp exp_stat_ipld_obj actual_stat_ipld_obj
  '

  test_expect_success "dag stat of simple UnixFS object" '
    BASIC_UNIXFS=$(echo "1234" | ipfs add --pin=false -q) &&
    ipfs dag stat $BASIC_UNIXFS > actual_stat_basic_unixfs &&
    echo "Size: 13, NumBlocks: 1" > exp_stat_basic_unixfs &&
    test_cmp exp_stat_basic_unixfs actual_stat_basic_unixfs
  '

  # The multiblock file is just 10000000 copies of the number 1
  # As most of its data is replicated it should have a small number of blocks
  test_expect_success "dag stat of multiblock UnixFS object" '
    MULTIBLOCK_UNIXFS=$(printf "1%.0s" {1..10000000} | ipfs add --pin=false -q) &&
    ipfs dag stat $MULTIBLOCK_UNIXFS > actual_stat_multiblock_unixfs &&
    echo "Size: 302582, NumBlocks: 3" > exp_stat_multiblock_unixfs &&
    test_cmp exp_stat_multiblock_unixfs actual_stat_multiblock_unixfs
  '

  test_expect_success "dag stat of directory of UnixFS objects" '
    mkdir -p unixfsdir &&
    echo "1234" > unixfsdir/small.txt
    printf "1%.0s" {1..10000000} > unixfsdir/many1s.txt &&
    DIRECTORY_UNIXFS=$(ipfs add -r --pin=false -Q unixfsdir) &&
    ipfs dag stat $DIRECTORY_UNIXFS > actual_stat_directory_unixfs &&
    echo "Size: 302705, NumBlocks: 5" > exp_stat_directory_unixfs &&
    test_cmp exp_stat_directory_unixfs actual_stat_directory_unixfs
  '
}

# should work offline
test_dag_cmd

# should work online
test_launch_ipfs_daemon
test_dag_cmd
test_kill_ipfs_daemon

test_done
