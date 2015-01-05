#!/bin/sh
#
# Copyright (c) 2014 Christian Couder
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test add and cat commands"

. lib/test-lib.sh

test_launch_ipfs_daemon_and_mount

test_expect_success "'ipfs add --help' succeeds" '
	ipfs add --help >actual
'

test_expect_success "'ipfs add --help' output looks good" '
	egrep "ipfs add.*<path>" actual >/dev/null ||
	fsh cat actual
'

test_expect_success "'ipfs cat --help' succeeds" '
	ipfs cat --help >actual
'

test_expect_success "'ipfs cat --help' output looks good" '
	egrep "ipfs cat.*<ipfs-path>" actual >/dev/null ||
	fsh cat actual
'

test_expect_success "ipfs add succeeds" '
	echo "Hello Worlds!" >mountdir/hello.txt &&
	ipfs add mountdir/hello.txt >actual
'

test_expect_success "ipfs add output looks good" '
	HASH="QmVr26fY1tKyspEJBniVhqxQeEjhF78XerGiqWAwraVLQH" &&
	echo "added $HASH mountdir/hello.txt" >expected &&
	test_cmp expected actual
'

test_expect_success "ipfs cat succeeds" '
	ipfs cat $HASH >actual
'

test_expect_success "ipfs cat output looks good" '
	echo "Hello Worlds!" >expected &&
	test_cmp expected actual
'

test_expect_success FUSE "cat ipfs/stuff succeeds" '
	cat ipfs/$HASH >actual
'

test_expect_success FUSE "cat ipfs/stuff looks good" '
	test_cmp expected actual
'

test_expect_success "'ipfs add -q' succeeds" '
	echo "Hello Venus!" >mountdir/venus.txt &&
	ipfs add -q mountdir/venus.txt >actual
'

test_expect_success "'ipfs add -q' output looks good" '
	HASH="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
	echo "$HASH" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs add -r' succeeds" '
	mkdir mountdir/planets &&
	echo "Hello Mars!" >mountdir/planets/mars.txt &&
	echo "Hello Venus!" >mountdir/planets/venus.txt &&
	ipfs add -r mountdir/planets >actual
'

test_expect_success "'ipfs add -r' output looks good" '
	VENUS="QmU5kp3BH3B8tnWUU2Pikdb2maksBNkb92FHRr56hyghh4" &&
	MARS="QmPrrHqJzto9m7SyiRzarwkqPcCSsKR2EB1AyqJfe8L8tN" &&
	PLANETS="QmPikaYYDyDNeZ4uGpDZJuA3EUgmq9ubhNHkXWWUrcwFz6" &&
	echo "added $PLANETS mountdir/planets" >expected &&
	echo "added $MARS mountdir/planets/mars.txt" >>expected &&
	echo "added $VENUS mountdir/planets/venus.txt" >>expected &&
	test_cmp expected actual
'

echo
echo "expected:"
cat expected

echo
echo "actual:"
cat actual

test_expect_success "go-random is installed" '
	type random
'

test_expect_success "generate 5MB file using go-random" '
	random 5242880 41 >mountdir/bigfile
'

test_expect_success "sha1 of the file looks ok" '
	echo "5620fb92eb5a49c9986b5c6844efda37e471660e  mountdir/bigfile" >sha1_expected &&
	shasum mountdir/bigfile >sha1_actual &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success "'ipfs add bigfile' succeeds" '
	ipfs add mountdir/bigfile >actual
'

test_expect_success "'ipfs add bigfile' output looks good" '
	HASH="Qmf2EnuvFQtpFnMJb5aoVPnMx9naECPSm8AGyktmEB5rrR" &&
	echo "added $HASH mountdir/bigfile" >expected &&
	test_cmp expected actual
'

test_expect_success "'ipfs cat' succeeds" '
	ipfs cat $HASH >actual
'

test_expect_success "'ipfs cat' output looks good" '
	test_cmp mountdir/bigfile actual
'

test_expect_success FUSE "cat ipfs/bigfile succeeds" '
	cat ipfs/$HASH >actual
'

test_expect_success FUSE "cat ipfs/bigfile looks good" '
	test_cmp mountdir/bigfile actual
'

test_expect_success EXPENSIVE "generate 100MB file using go-random" '
	random 104857600 42 >mountdir/bigfile
'

test_expect_success EXPENSIVE "sha1 of the file looks ok" '
	echo "885b197b01e0f7ff584458dc236cb9477d2e736d  mountdir/bigfile" >sha1_expected &&
	shasum mountdir/bigfile >sha1_actual &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success EXPENSIVE "ipfs add bigfile succeeds" '
	ipfs add mountdir/bigfile >actual
'

test_expect_success EXPENSIVE "ipfs add bigfile output looks good" '
	HASH="QmWXysX1oysyjTqd5xGM2T1maBaVXnk5svQv4GKo5PsGPo" &&
	echo "added $HASH mountdir/bigfile" >expected &&
	test_cmp expected actual
'

test_expect_success EXPENSIVE "ipfs cat succeeds" '
	ipfs cat $HASH | shasum >sha1_actual
'

test_expect_success EXPENSIVE "ipfs cat output looks good" '
	ipfs cat $HASH >actual &&
	test_cmp mountdir/bigfile actual
'

test_expect_success EXPENSIVE "ipfs cat output shasum looks good" '
	echo "885b197b01e0f7ff584458dc236cb9477d2e736d  -" >sha1_expected &&
	test_cmp sha1_expected sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile succeeds" '
	cat ipfs/$HASH | shasum >sha1_actual
'

test_expect_success FUSE,EXPENSIVE "cat ipfs/bigfile looks good" '
	test_cmp sha1_expected sha1_actual
'

test_kill_ipfs_daemon

test_done
