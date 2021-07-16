#!/usr/bin/env bash
#
# Copyright (c) 2017 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test out the urlstore functionality"

. lib/test-lib.sh


test_expect_success "create some random files" '
  random 2222     7 > file1 &&
  random 500000   7 > file2 &&
  random 50000000 7 > file3
'

test_urlstore() {
  ADD_CMD="${@}"

  test_init_ipfs
  
  test_expect_success "add files using trickle dag format without raw leaves" '
    HASH1a=$(ipfs add -q --trickle --raw-leaves=false file1) &&
    HASH2a=$(ipfs add -q --trickle --raw-leaves=false file2) &&
    HASH3a=$(ipfs add -q --trickle --raw-leaves=false file3)
  '
  
  test_launch_ipfs_daemon_without_network
  
  test_expect_success "make sure files can be retrieved via the gateway" '
    curl http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a -o file1.actual &&
    test_cmp file1 file1.actual &&
    curl http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a -o file2.actual &&
    test_cmp file2 file2.actual &&
    curl http://127.0.0.1:$GWAY_PORT/ipfs/$HASH3a -o file3.actual &&
    test_cmp file3 file3.actual 
  '
  
  test_expect_success "add files without enabling url store using $ADD_CMD" '
    test_must_fail ipfs $ADD_CMD http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a &&
    test_must_fail ipfs $ADD_CMD http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a
  '
  
  test_kill_ipfs_daemon
  
  test_expect_success "enable urlstore" '
    ipfs config --json Experimental.UrlstoreEnabled true
  '
  
  test_launch_ipfs_daemon_without_network
  
  test_expect_success "add files using gateway address via url store using $ADD_CMD" '
    HASH1=$(ipfs $ADD_CMD --pin=false http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a) &&
    HASH2=$(ipfs $ADD_CMD http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a)
  '
  
  test_expect_success "make sure hashes are different" '
    test $HASH1a != $HASH1 &&
    test $HASH2a != $HASH2
  '
  
  test_expect_success "get files via urlstore" '
    rm -f file1.actual file2.actual &&
    ipfs get $HASH1 -o file1.actual &&
    test_cmp file1 file1.actual &&
    ipfs get $HASH2 -o file2.actual &&
    test_cmp file2 file2.actual
  '

  cat <<EOF | sort > ls_expect
bafkreiafqvawjpukk4achpu7edu4d6x5dbzwgigl6nxunjif3ser6bnfpu 262144 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 0
bafkreia46t3jwchosehfcq7kponx26shcjkatxek4m2tzzd67i6o3frpou 237856 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 262144
bafkreiga7ukbxrxs26fiseijjd7zdd6gmlrmnxhalwfbagxwjv7ck4o34a   2222 http://127.0.0.1:$GWAY_PORT/ipfs/QmcHm3BL2cXuQ6rJdKQgPrmT9suqGkfy2KzH3MkXPEBXU6 0
EOF

  test_expect_success "ipfs filestore ls works with urls" '
    ipfs filestore ls | sort > ls_actual &&
    test_cmp ls_expect ls_actual
  '

  cat <<EOF | sort > verify_expect
ok      bafkreiafqvawjpukk4achpu7edu4d6x5dbzwgigl6nxunjif3ser6bnfpu 262144 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 0
ok      bafkreia46t3jwchosehfcq7kponx26shcjkatxek4m2tzzd67i6o3frpou 237856 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 262144
ok      bafkreiga7ukbxrxs26fiseijjd7zdd6gmlrmnxhalwfbagxwjv7ck4o34a   2222 http://127.0.0.1:$GWAY_PORT/ipfs/QmcHm3BL2cXuQ6rJdKQgPrmT9suqGkfy2KzH3MkXPEBXU6 0
EOF
  
  test_expect_success "ipfs filestore verify works with urls" '
    ipfs filestore verify | sort > verify_actual &&
    test_cmp verify_expect verify_actual
  '
  
  test_expect_success "garbage collect file1 from the urlstore" '
    ipfs repo gc > /dev/null
  '
  
  test_expect_success "can no longer retrieve file1 from urlstore" '
    rm -f file1.actual &&
    test_must_fail ipfs get $HASH1 -o file1.actual
  '
  
  test_expect_success "can still retrieve file2 from urlstore" '
    rm -f file2.actual &&
    ipfs get $HASH2 -o file2.actual &&
    test_cmp file2 file2.actual
  '
  
  test_expect_success "remove original hashes from local gateway" '
    ipfs pin rm $HASH1a $HASH2a &&
    ipfs repo gc > /dev/null
  '
  
  test_expect_success "gatway no longer has files" '
    test_must_fail curl -f http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a -o file1.actual
    test_must_fail curl -f http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a -o file2.actual
  '

  cat <<EOF | sort > verify_expect_2
error   bafkreiafqvawjpukk4achpu7edu4d6x5dbzwgigl6nxunjif3ser6bnfpu 262144 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 0
error   bafkreia46t3jwchosehfcq7kponx26shcjkatxek4m2tzzd67i6o3frpou 237856 http://127.0.0.1:$GWAY_PORT/ipfs/QmUow2T4P69nEsqTQDZCt8yg9CPS8GFmpuDAr5YtsPhTdM 262144
EOF
  
  test_expect_success "ipfs filestore verify is correct" '
    ipfs filestore verify | sort > verify_actual_2 &&
    test_cmp verify_expect_2 verify_actual_2
  '
  
  test_expect_success "files cannot be retrieved via the urlstore" '
    test_must_fail ipfs cat $HASH1 > /dev/null &&
    test_must_fail ipfs cat $HASH2 > /dev/null
  '
  
  test_expect_success "remove broken files" '
    ipfs pin rm $HASH2 &&
    ipfs repo gc > /dev/null
  '
  
  test_expect_success "add large file using gateway address via url store" '
    HASH3=$(ipfs ${ADD_CMD[@]} http://127.0.0.1:$GWAY_PORT/ipfs/$HASH3a)
  '
  
  test_expect_success "make sure hashes are different" '
    test $HASH3a != $HASH3
  '
  
  test_expect_success "get large file via urlstore" '
    rm -f file3.actual &&
    ipfs get $HASH3 -o file3.actual &&
    test_cmp file3 file3.actual
  '
  
  test_expect_success "check that the trickle option works" '
    HASHat=$(ipfs add -q --cid-version=1 --raw-leaves=true -n --trickle file3) &&
    HASHut=$(ipfs $ADD_CMD --trickle http://127.0.0.1:$GWAY_PORT/ipfs/$HASH3a) &&
    test $HASHat = $HASHut
  '
  
  test_expect_success "add files using gateway address via url store using --cid-base=base32" '
    HASH1a=$(ipfs add -q --trickle --raw-leaves=false file1) &&
    HASH2a=$(ipfs add -q --trickle --raw-leaves=false file2) &&
    HASH1b32=$(ipfs --cid-base=base32 $ADD_CMD http://127.0.0.1:$GWAY_PORT/ipfs/$HASH1a) &&
    HASH2b32=$(ipfs --cid-base=base32 $ADD_CMD http://127.0.0.1:$GWAY_PORT/ipfs/$HASH2a)
  '
  
  test_kill_ipfs_daemon
  
  test_expect_success "files cannot be retrieved via the urlstore" '
    test_must_fail ipfs cat $HASH1 > /dev/null &&
    test_must_fail ipfs cat $HASH2 > /dev/null &&
    test_must_fail ipfs cat $HASH3 > /dev/null
  '
  
  test_expect_success "check that the hashes were correct" '
    HASH1e=$(ipfs add -q -n --cid-version=1 --raw-leaves=true file1) &&
    HASH2e=$(ipfs add -q -n --cid-version=1 --raw-leaves=true file2) &&
    HASH3e=$(ipfs add -q -n --cid-version=1 --raw-leaves=true file3) &&
    test $HASH1e = $HASH1 &&
    test $HASH2e = $HASH2 &&
    test $HASH3e = $HASH3
  '
  
  test_expect_success "check that the base32 hashes were correct" '
    HASH1e32=$(ipfs cid base32 $HASH1e)
    HASH2e32=$(ipfs cid base32 $HASH2e)
    test $HASH1e32 = $HASH1b32 &&
    test $HASH2e32 = $HASH2b32
  '

  test_expect_success "ipfs cleanup" '
    rm -rf "$IPFS_PATH" && rmdir ipfs ipns mountdir
  '
}

test_urlstore urlstore add
test_urlstore add -q --nocopy --cid-version=1

test_done
