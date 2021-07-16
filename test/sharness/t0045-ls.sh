#!/usr/bin/env bash
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test ls command"

. lib/test-lib.sh

test_init_ipfs

test_ls_cmd() {
  test_expect_success "'ipfs add -r testData' succeeds" '
    mkdir -p testData testData/d1 testData/d2 &&
    echo "test" >testData/f1 &&
    echo "data" >testData/f2 &&
    echo "hello" >testData/d1/a &&
    random 128 42 >testData/d1/128 &&
    echo "world" >testData/d2/a &&
    random 1024 42 >testData/d2/1024 &&
    echo "badname" >testData/d2/`echo -e "bad\x7fname.txt"` &&
    ipfs add -r testData >actual_add
  '

  test_expect_success "'ipfs add' output looks good" '
    cat <<-\EOF >expected_add &&
added QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe testData/d1/128
added QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN testData/d1/a
added QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd testData/d2/1024
added QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL testData/d2/a
added QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn testData/d2/bad\u007fname.txt
added QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH testData/f1
added QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M testData/f2
added QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss testData/d1
added Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy testData/d2
added QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 testData
EOF
    test_cmp expected_add actual_add
  '

  test_expect_success "'ipfs ls <three dir hashes>' succeeds" '
    ipfs ls QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls
  '

  test_expect_success "'ipfs ls <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss - d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy - d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5 f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5 f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6    a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8    bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6   a
EOF
    test_cmp expected_ls actual_ls
  '

  test_expect_success "'ipfs ls --size=false <three dir hashes>' succeeds" '
    ipfs ls --size=false QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls
  '

  test_expect_success "'ipfs ls <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN a
EOF
    test_cmp expected_ls actual_ls
  '

  test_expect_success "'ipfs ls --headers <three dir hashes>' succeeds" '
    ipfs ls --headers QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls_headers
  '

  test_expect_success "'ipfs ls --headers  <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_headers &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
Hash                                           Size Name
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss -    d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy -    d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5    f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5    f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
Hash                                           Size Name
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6    a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8    bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
Hash                                           Size Name
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128  128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6    a
EOF
    test_cmp expected_ls_headers actual_ls_headers
  '

  test_expect_success "'ipfs ls --size=false --cid-base=base32 <three dir hashes>' succeeds" '
    ipfs ls --size=false --cid-base=base32 $(cid-fmt -v 1 -b base32 %s QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss) >actual_ls_base32
  '

  test_expect_success "'ipfs ls --size=false --cid-base=base32 <three dir hashes>' output looks good" '
    cid-fmt -b base32 -v 1 --filter %s < expected_ls > expected_ls_base32
    test_cmp expected_ls_base32 actual_ls_base32
  '
}


test_ls_cmd_streaming() {

  test_expect_success "'ipfs add -r testData' succeeds" '
    mkdir -p testData testData/d1 testData/d2 &&
    echo "test" >testData/f1 &&
    echo "data" >testData/f2 &&
    echo "hello" >testData/d1/a &&
    random 128 42 >testData/d1/128 &&
    echo "world" >testData/d2/a &&
    random 1024 42 >testData/d2/1024 &&
    echo "badname" >testData/d2/`echo -e "bad\x7fname.txt"` &&
    ipfs add -r testData >actual_add
  '

  test_expect_success "'ipfs add' output looks good" '
    cat <<-\EOF >expected_add &&
added QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe testData/d1/128
added QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN testData/d1/a
added QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd testData/d2/1024
added QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL testData/d2/a
added QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn testData/d2/bad\u007fname.txt
added QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH testData/f1
added QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M testData/f2
added QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss testData/d1
added Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy testData/d2
added QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 testData
EOF
    test_cmp expected_add actual_add
  '

  test_expect_success "'ipfs ls --stream <three dir hashes>' succeeds" '
    ipfs ls --stream QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls_stream
  '

  test_expect_success "'ipfs ls --stream <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss -         d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy -         d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5         f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5         f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024      1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6         a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8         bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128       128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6         a
EOF
    test_cmp expected_ls_stream actual_ls_stream
  '

  test_expect_success "'ipfs ls --size=false --stream <three dir hashes>' succeeds" '
    ipfs ls --size=false --stream QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls_stream
  '

  test_expect_success "'ipfs ls --size=false --stream <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN a
EOF
    test_cmp expected_ls_stream actual_ls_stream
  '

  test_expect_success "'ipfs ls --stream --headers <three dir hashes>' succeeds" '
    ipfs ls --stream --headers QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21 Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss >actual_ls_stream_headers
  '

  test_expect_success "'ipfs ls --stream --headers  <three dir hashes>' output looks good" '
    cat <<-\EOF >expected_ls_stream_headers &&
QmRPX2PWaPGqzoVzqNcQkueijHVzPicjupnD7eLck6Rs21:
Hash                                           Size      Name
QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss -         d1/
Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy -         d2/
QmeomffUNfmQy76CQGy9NdmqEnnHU9soCexBnGU3ezPHVH 5         f1
QmNtocSs7MoDkJMc1RkyisCSKvLadujPsfJfSdJ3e1eA1M 5         f2

Qmf9nCpkCfa8Gtz5m1NJMeHBWcBozKRcbdom338LukPAjy:
Hash                                           Size      Name
QmbQBUSRL9raZtNXfpTDeaxQapibJEG6qEY8WqAN22aUzd 1024      1024
QmaRGe7bVmVaLmxbrMiVNXqW4pRNNp3xq7hFtyRKA3mtJL 6         a
QmQSLRRd1Lxn6NMsWmmj2g9W3LtSRfmVAVqU3ShneLUrbn 8         bad\u007fname.txt

QmSix55yz8CzWXf5ZVM9vgEvijnEeeXiTSarVtsqiiCJss:
Hash                                           Size      Name
QmQNd6ubRXaNG6Prov8o6vk3bn6eWsj9FxLGrAVDUAGkGe 128       128
QmZULkCELmmk5XNfCgTnCyFgAVxBRBXyDHGGMVoLFLiXEN 6         a
EOF
    test_cmp expected_ls_stream_headers actual_ls_stream_headers
  '
}

test_ls_cmd_raw_leaves() {
  test_expect_success "'ipfs add -r --raw-leaves' then 'ipfs ls' works as expected" '
    mkdir -p somedir &&
    echo bar > somedir/foo &&
    ipfs add --raw-leaves -r somedir/ > /dev/null &&
    ipfs ls '$1' QmThNTdtKaVoCVrYmM5EBS6U3S5vfKFue2TxbxxAxRcKKE > ls-actual
    echo "bafkreid5qzpjlgzem2iyzgddv7fjilipxcoxzgwazgn27q3usucn5wlxga 4 foo" > ls-expect
    test_cmp ls-actual ls-expect
  '
}

test_ls_object() {
  test_expect_success "ipfs add medium size file then 'ipfs ls --size=false' works as expected" '
    random 500000 2 > somefile &&
    HASH=$(ipfs add somefile -q) &&
    echo "QmPrM8S5T7Q3M8DQvQMS7m41m3Aq4jBjzAzvky5fH3xfr4 " > ls-expect &&
    echo "QmdaAntAzQqqVMo4B8V69nkQd5d918YjHXUe2oF6hr72ri " >> ls-expect &&
    ipfs ls --size=false $HASH > ls-actual &&
    test_cmp ls-actual ls-expect
  '

  test_expect_success "ipfs add medium size file then 'ipfs ls' works as expected" '
    random 500000 2 > somefile &&
    HASH=$(ipfs add somefile -q) &&
    echo "QmPrM8S5T7Q3M8DQvQMS7m41m3Aq4jBjzAzvky5fH3xfr4 262144 " > ls-expect &&
    echo "QmdaAntAzQqqVMo4B8V69nkQd5d918YjHXUe2oF6hr72ri 237856 " >> ls-expect &&
    ipfs ls $HASH > ls-actual &&
    test_cmp ls-actual ls-expect
  '
}

# should work offline
test_ls_cmd
test_ls_cmd_streaming
test_ls_cmd_raw_leaves
test_ls_cmd_raw_leaves --size
test_ls_object

# should work online
test_launch_ipfs_daemon
test_ls_cmd
test_ls_cmd_streaming
test_ls_cmd_raw_leaves
test_ls_cmd_raw_leaves --size
test_kill_ipfs_daemon
test_ls_object

#
# test for ls --resolve-type=false
#

test_expect_success "'ipfs add -r' succeeds" '
  mkdir adir &&
  # note: not using a seed as the files need to have truly random content
  random 1000 > adir/file1 &&
  random 1000 > adir/file2 &&
  ipfs add --pin=false -q -r adir > adir-hashes
'

test_expect_success "get hashes from add output" '
  FILE=`head -1 adir-hashes` &&
  DIR=`tail -1 adir-hashes` &&
  test "$FILE" -a "$DIR"
'

test_expect_success "remove a file in dir" '
  ipfs block rm $FILE
'

test_expect_success "'ipfs ls --resolve-type=false ' fails" '
  test_must_fail ipfs ls --resolve-type=false $DIR > /dev/null
'

test_expect_success "'ipfs ls' fails" '
  test_must_fail ipfs ls $DIR
'

test_expect_success "'ipfs ls --resolve-type=true --size=false' fails" '
  test_must_fail ipfs ls --resolve-type=true --size=false $DIR
'

test_launch_ipfs_daemon_without_network

test_expect_success "'ipfs ls --resolve-type=false --size=false' ok" '
  ipfs ls --resolve-type=false --size=false $DIR > /dev/null
'

test_expect_success "'ipfs ls' fails" '
  test_must_fail ipfs ls $DIR
'

test_expect_success "'ipfs ls --resolve-type=false --size=true' fails" '
  test_must_fail ipfs ls --resolve-type=false --size=true $DIR
'

test_kill_ipfs_daemon

test_launch_ipfs_daemon

# now we try `ipfs ls --resolve-type=false` with the daemon online It
# should not even attempt to retrieve the file from the network.  If
# it does it should eventually fail as the content is random and
# should not exist on the network, but we don't want to wait for a
# timeout so we will kill the request after a few seconds
test_expect_success "'ipfs ls --resolve-type=false --size=false' ok and does not hang" '
  go-timeout 2 ipfs ls --resolve-type=false --size=false $DIR
'

test_kill_ipfs_daemon

test_done
