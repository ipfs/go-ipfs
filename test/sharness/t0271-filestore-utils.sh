#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test out the filestore nocopy functionality"

. lib/test-lib.sh

test_init_filestore() {
  test_expect_success "clean up old node" '
    rm -rf "$IPFS_PATH" mountdir ipfs ipns
  '

  test_init_ipfs

  test_expect_success "enable filestore config setting" '
    ipfs config --json Experimental.FilestoreEnabled true
  '
}

test_init_dataset() {
  test_expect_success "create a dataset" '
    rm -r somedir
    mkdir somedir &&
    random    1000  1 > somedir/file1 &&
    random   10000  2 > somedir/file2 &&
    random 1000000  3 > somedir/file3
  '
}

test_init() {
  test_init_filestore
  test_init_dataset
}

EXPHASH="QmRueCuPMYYvdxWz1vWncF7wzCScEx4qasZXo5aVBb1R4V"

cat <<EOF > ls_expect_file_order
bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq   1000 somedir/file1 0
bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey  10000 somedir/file2 0
bafkreidntk6ciin24oez6yjz4b25fgwecncvi4ua4uhr2tdyenogpzpid4 262144 somedir/file3 0
bafkreidwie26yauqbhpd2nhhhmod55irq3z372mh6gw4ikl2ifo34c5jra 262144 somedir/file3 262144
bafkreib7piyesy3dr22sawmycdftrmpyt3z4tmhxrdig2zt5zdp7qwbuay 262144 somedir/file3 524288
bafkreigxp5k3k6b3i5sldu4r3im74nfxmoptuuubcvq6rg632nfznskglu 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq
FILE2_HASH=bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey
FILE3_HASH=QmfE4SDQazxTD7u8VTYs9AJqQL8rrJPUAorLeJXKSZrVf9

cat <<EOF > verify_expect_file_order
ok      bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq   1000 somedir/file1 0
ok      bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey  10000 somedir/file2 0
ok      bafkreidntk6ciin24oez6yjz4b25fgwecncvi4ua4uhr2tdyenogpzpid4 262144 somedir/file3 0
ok      bafkreidwie26yauqbhpd2nhhhmod55irq3z372mh6gw4ikl2ifo34c5jra 262144 somedir/file3 262144
ok      bafkreib7piyesy3dr22sawmycdftrmpyt3z4tmhxrdig2zt5zdp7qwbuay 262144 somedir/file3 524288
ok      bafkreigxp5k3k6b3i5sldu4r3im74nfxmoptuuubcvq6rg632nfznskglu 213568 somedir/file3 786432
EOF

sort < verify_expect_file_order > verify_expect_key_order

IPFS_CMD="ipfs"

test_filestore_adds() {
  test_expect_success "$IPFS_CMD add nocopy add succeeds" '
    HASH=$($IPFS_CMD add --raw-leaves --nocopy -r -q somedir | tail -n1)
  '

  test_expect_success "nocopy add has right hash" '
    test "$HASH" = "$EXPHASH"
  '

  test_expect_success "'$IPFS_CMD filestore ls' output looks good'" '
    $IPFS_CMD filestore ls | sort > ls_actual &&
    test_cmp ls_expect_key_order ls_actual
  '

  test_expect_success "'$IPFS_CMD filestore ls --file-order' output looks good'" '
    $IPFS_CMD filestore ls --file-order > ls_actual &&
    test_cmp ls_expect_file_order ls_actual
  '

  test_expect_success "'$IPFS_CMD filestore ls HASH' works" '
    $IPFS_CMD filestore ls $FILE1_HASH > ls_actual &&
    grep -q somedir/file1 ls_actual
  '

  test_expect_success "can retrieve multi-block file" '
    $IPFS_CMD cat $FILE3_HASH > file3.data &&
    test_cmp somedir/file3 file3.data
  '
}

# check that the filestore is in a clean state
test_filestore_state() {
  test_expect_success "$IPFS_CMD filestore verify' output looks good'" '
    $IPFS_CMD filestore verify | LC_ALL=C sort > verify_actual
    test_cmp verify_expect_key_order verify_actual
  '
}

test_filestore_verify() {
  test_filestore_state

  test_expect_success "$IPFS_CMD filestore verify --file-order' output looks good'" '
    $IPFS_CMD filestore verify --file-order > verify_actual
    test_cmp verify_expect_file_order verify_actual
  '

  test_expect_success "'$IPFS_CMD filestore verify HASH' works" '
    $IPFS_CMD filestore verify $FILE1_HASH > verify_actual &&
    grep -q somedir/file1 verify_actual
  '

  test_expect_success "rename a file" '
    mv somedir/file1 somedir/file1.bk
  '

  test_expect_success "can not retrieve block after backing file moved" '
    test_must_fail $IPFS_CMD cat $FILE1_HASH
  '

  test_expect_success "'$IPFS_CMD filestore verify' shows file as missing" '
    $IPFS_CMD filestore verify > verify_actual &&
    grep no-file verify_actual | grep -q somedir/file1
  '

  test_expect_success "move file back" '
    mv somedir/file1.bk somedir/file1
  '

  test_expect_success "block okay now" '
    $IPFS_CMD cat $FILE1_HASH > file1.data &&
    test_cmp somedir/file1 file1.data
  '

  test_expect_success "change first bit of file" '
    dd if=/dev/zero of=somedir/file3 bs=1024 count=1
  '

  test_expect_success "can not retrieve block after backing file changed" '
    test_must_fail $IPFS_CMD cat $FILE3_HASH
  '

  test_expect_success "'$IPFS_CMD filestore verify' shows file as changed" '
    $IPFS_CMD filestore verify > verify_actual &&
    grep changed verify_actual | grep -q somedir/file3
  '

  # reset the state for the next test
  test_init_dataset
}

test_filestore_dups() {
  # make sure the filestore is in a clean state
  test_filestore_state

  test_expect_success "'$IPFS_CMD filestore dups'" '
    $IPFS_CMD add --raw-leaves somedir/file1 &&
    $IPFS_CMD filestore dups > dups_actual &&
    echo "$FILE1_HASH" > dups_expect
    test_cmp dups_expect dups_actual
  '
}

#
# No daemon
#

test_init

test_filestore_adds

test_filestore_verify

test_filestore_dups

#
# With daemon
#

test_init

# must be in offline mode so tests that retrieve non-existent blocks
# doesn't hang
test_launch_ipfs_daemon_without_network

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_kill_ipfs_daemon

##
## base32
##

EXPHASH="bafybeibva2uh4qpwjo2yr5g7m7nd5kfq64atydq77qdlrikh5uejwqdcbi"

cat <<EOF > ls_expect_file_order
bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq   1000 somedir/file1 0
bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey  10000 somedir/file2 0
bafkreidntk6ciin24oez6yjz4b25fgwecncvi4ua4uhr2tdyenogpzpid4 262144 somedir/file3 0
bafkreidwie26yauqbhpd2nhhhmod55irq3z372mh6gw4ikl2ifo34c5jra 262144 somedir/file3 262144
bafkreib7piyesy3dr22sawmycdftrmpyt3z4tmhxrdig2zt5zdp7qwbuay 262144 somedir/file3 524288
bafkreigxp5k3k6b3i5sldu4r3im74nfxmoptuuubcvq6rg632nfznskglu 213568 somedir/file3 786432
EOF

sort < ls_expect_file_order > ls_expect_key_order

FILE1_HASH=bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq
FILE2_HASH=bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey
FILE3_HASH=bafybeih24zygzr2orr5q62mjnbgmjwgj6rx3tp74pwcqsqth44rloncllq

cat <<EOF > verify_expect_file_order
ok      bafkreicj3ezgtrh3euw2gyub6w3jydhnouqobxt7stbgtns3mv3iwv6bqq   1000 somedir/file1 0
ok      bafkreibxwxisv4cld6x76ybqbvf2uwbkoswjqt4hut46af6rps2twme7ey  10000 somedir/file2 0
ok      bafkreidntk6ciin24oez6yjz4b25fgwecncvi4ua4uhr2tdyenogpzpid4 262144 somedir/file3 0
ok      bafkreidwie26yauqbhpd2nhhhmod55irq3z372mh6gw4ikl2ifo34c5jra 262144 somedir/file3 262144
ok      bafkreib7piyesy3dr22sawmycdftrmpyt3z4tmhxrdig2zt5zdp7qwbuay 262144 somedir/file3 524288
ok      bafkreigxp5k3k6b3i5sldu4r3im74nfxmoptuuubcvq6rg632nfznskglu 213568 somedir/file3 786432
EOF

sort < verify_expect_file_order > verify_expect_key_order

IPFS_CMD="ipfs --cid-base=base32"

#
# No daemon
#

test_init

test_filestore_adds

test_filestore_verify

test_filestore_dups

#
# With daemon
#

test_init

# must be in offline mode so tests that retrieve non-existent blocks
# doesn't hang
test_launch_ipfs_daemon_without_network

test_filestore_adds

test_filestore_verify

test_filestore_dups

test_kill_ipfs_daemon

test_done

##

test_done
