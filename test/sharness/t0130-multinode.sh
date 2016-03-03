#!/bin/sh
#
# Copyright (c) 2015 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test multiple ipfs nodes"

. lib/test-lib.sh

check_file_fetch() {
	node=$1
	fhash=$2
	fname=$3

	test_expect_success "can fetch file" '
		ipfsi $node cat $fhash > fetch_out
	'

	test_expect_success "file looks good" '
		test_cmp $fname fetch_out
	'
}

run_basic_test() {
	startup_cluster 5

	test_expect_success "add a file on node1" '
		random 1000000 > filea &&
		FILEA_HASH=$(ipfsi 1 add -q filea)
	'

	check_file_fetch 4 $FILEA_HASH filea
	check_file_fetch 3 $FILEA_HASH filea
	check_file_fetch 2 $FILEA_HASH filea
	check_file_fetch 1 $FILEA_HASH filea
	check_file_fetch 0 $FILEA_HASH filea

	test_expect_success "shut down nodes" '
		iptb stop
	'
}

test_expect_success "set up tcp testbed" '
	iptb init -n 5 -p 0 -f --bootstrap=none
'

run_basic_test

test_expect_success "set up utp testbed" '
	iptb init -n 5 -p 0 -f --bootstrap=none --utp
'

run_basic_test

test_done
