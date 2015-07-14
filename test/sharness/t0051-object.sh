#!/bin/sh
#
# Copyright (c) 2015 Henry Bubert
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test object command"

. lib/test-lib.sh

test_init_ipfs

test_object_cmd() {

	test_expect_success "'ipfs add testData' succeeds" '
		printf "Hello Mars" >expected_in &&
		ipfs add expected_in >actual_Addout
	'
	
	test_expect_success "'ipfs add testData' output looks good" '
		HASH="QmWkHFpYBZ9mpPRreRbMhhYWXfUhBAue3JkbbpFqwowSRb" &&
		echo "added $HASH expected_in" >expected_Addout &&
		test_cmp expected_Addout actual_Addout
	'
	
	test_expect_success "'ipfs object get' succeeds" '
		ipfs object get $HASH >actual_getOut
	'
	
	test_expect_success "'ipfs object get' output looks good" '
		test_cmp ../t0051-object-data/expected_getOut actual_getOut
	'
	
	test_expect_success "'ipfs object stat' succeeds" '
		ipfs object stat $HASH >actual_stat
	'
	
	test_expect_success "'ipfs object get' output looks good" '
		echo "NumLinks: 0" > expected_stat &&
		echo "BlockSize: 18" >> expected_stat &&
		echo "LinksSize: 2" >> expected_stat &&
		echo "DataSize: 16" >> expected_stat &&
		echo "CumulativeSize: 18" >> expected_stat &&
		test_cmp expected_stat actual_stat
	'
	
	test_expect_success "'ipfs object put file.json' succeeds" '
		ipfs object put  ../t0051-object-data/testPut.json > actual_putOut
	'
	
	test_expect_success "'ipfs object put file.json' output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putOut &&
		test_cmp expected_putOut actual_putOut
	'
	
	test_expect_success "'ipfs object put file.pb' succeeds" '
		ipfs object put --inputenc=protobuf ../t0051-object-data/testPut.pb > actual_putOut
	'
	
	test_expect_success "'ipfs object put file.pb' output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putOut &&
		test_cmp expected_putOut actual_putOut
	'
	
	test_expect_success "'ipfs object put' from stdin succeeds" '
		cat ../t0051-object-data/testPut.json | ipfs object put > actual_putStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putStdinOut &&
		test_cmp expected_putStdinOut actual_putStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin (pb) succeeds" '
		cat ../t0051-object-data/testPut.pb | ipfs object put --inputenc=protobuf > actual_putPbStdinOut
	'
	
	test_expect_success "'ipfs object put' from stdin (pb) output looks good" '
		HASH="QmUTSAdDi2xsNkDtLqjFgQDMEn5di3Ab9eqbrt4gaiNbUD" &&
		printf "added $HASH" > expected_putStdinOut &&
		test_cmp expected_putStdinOut actual_putPbStdinOut
	'
	
	test_expect_success "'ipfs object put broken.json' should fail" '
		test_expect_code 1 ipfs object put ../t0051-object-data/brokenPut.json 2>actual_putBrokenErr >actual_putBroken
	'
	
	test_expect_success "'ipfs object put broken.hjson' output looks good" '
		touch expected_putBroken &&
		printf "Error: no data or links in this node\n" > expected_putBrokenErr &&
		test_cmp expected_putBroken actual_putBroken &&
		test_cmp expected_putBrokenErr actual_putBrokenErr
	'

	test_expect_success "'ipfs object patch' should work" '
		EMPTY_DIR=$(ipfs object new unixfs-dir) &&
		OUTPUT=$(ipfs object patch $EMPTY_DIR add-link foo $EMPTY_DIR)
	'

	test_expect_success "should have created dir within a dir" '
		ipfs ls $OUTPUT > patched_output
	'

	test_expect_success "output looks good" '
		echo "QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn 4 foo/" > patched_exp &&
		test_cmp patched_exp patched_output
	'

	test_expect_success "object patch add-link can create links with existing names" '
		EMPTY=$(ipfs object new) &&
		P1=$(ipfs object patch $EMPTY add-link foo $EMPTY) &&
		ipfs object patch $P1 add-link foo $EMPTY >multiple_add_links
	'

	test_expect_success "object patch add-link with existing names looks good" '
		cat <<-\EOF >multiple_add_links_expected_newline &&
			{
			  "Links": [
			    {
			      "Name": "foo",
			      "Hash": "QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n",
			      "Size": 0
			    },
			    {
			      "Name": "foo",
			      "Hash": "QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n",
			      "Size": 0
			    }
			  ],
			  "Data": ""
			}
		EOF
		printf %s "$(<multiple_add_links_expected_newline)" >multiple_add_links_expected &&
		ipfs object get $(<multiple_add_links) >multiple_add_links_output &&
		test_cmp multiple_add_links_expected multiple_add_links_output
	'

	test_expect_success "object patch replace-link overwrites existing names" '
		EMPTY=$(ipfs object new) &&
		EMPTY_DIR=$(ipfs object new unixfs-dir) &&
		P1=$(ipfs object patch $EMPTY replace-link foo $EMPTY_DIR) &&
		ipfs object patch $P1 replace-link foo $EMPTY >multiple_replace_links
	'

	test_expect_success "object patch replace-link with existing names looks good" '
		cat <<-\EOF >multiple_replace_links_expected_newline &&
			{
			  "Links": [
			    {
			      "Name": "foo",
			      "Hash": "QmdfTbBqBPQ7VNxZEYEj14VmRuZBkqFbiwReogJgS1zR1n",
			      "Size": 0
			    }
			  ],
			  "Data": ""
			}
		EOF
		printf %s "$(<multiple_replace_links_expected_newline)" >multiple_replace_links_expected &&
		ipfs object get $(<multiple_replace_links) >multiple_replace_links_output &&
		test_cmp multiple_replace_links_expected multiple_replace_links_output
	'

	test_expect_success "can remove the directory" '
		ipfs object patch $OUTPUT rm-link foo > rmlink_output
	'

	test_expect_success "output should be empty" '
		echo QmUNLLsPACCz1vLxQVkXqqLX5R1X345qqfHbsf67hvA3Nn > rmlink_exp &&
		test_cmp rmlink_exp rmlink_output
	'

	test_expect_success "object patch rm-link removes multiple links" '
		EMPTY=$(ipfs object new) &&
		P1=$(ipfs object patch $EMPTY add-link foo $EMPTY) &&
		P2=$(ipfs object patch $P1 add-link foo $EMPTY) &&
		ipfs object patch $P2 rm-link foo >rm_multiple_links
	'

	test_expect_success "object patch rm-link multi-link removal looks good" '
		cat <<-\EOF >rm_multiple_links_expected_newline &&
			{
			  "Links": [],
			  "Data": ""
			}
		EOF
		printf %s "$(<rm_multiple_links_expected_newline)" >rm_multiple_links_expected &&
		ipfs object get $(<rm_multiple_links) >rm_multiple_links_actual &&
		test_cmp rm_multiple_links_expected rm_multiple_links_actual
	'

	test_expect_success "'ipfs object patch set-data' should work" '
		EMPTY=$(ipfs object new) &&
		printf %s "hello world" >set_data_expected &&
		PATCHED=$(ipfs object patch $EMPTY set-data "$(<set_data_expected)") &&
		ipfs object data $PATCHED >set_data &&
		test_cmp set_data_expected set_data
	'

	test_expect_success "'ipfs object patch set-data' should overwrite existing data" '
		EMPTY_DIR=$(ipfs object new unixfs-dir) &&
		printf %s "hello world" >set_data_overwrite_expected &&
		PATCHED=$(ipfs object patch $EMPTY_DIR set-data "$(<set_data_overwrite_expected)") &&
		ipfs object data $PATCHED >set_data_overwrite &&
		test_cmp set_data_overwrite_expected set_data_overwrite
	'

	test_expect_success "'ipfs object patch append-data' should work" '
		EMPTY=$(ipfs object new) &&
		printf %s "hello world" >empty_append_data_expected &&
		PATCHED=$(ipfs object patch $EMPTY append-data "$(<empty_append_data_expected)") &&
		ipfs object data $PATCHED >empty_append_data &&
		test_cmp empty_append_data_expected empty_append_data
	'

	test_expect_success "'ipfs object patch append-data' should append to existing data" '
		EMPTY=$(ipfs object new) &&
		printf %s "hello world" >append_data_expected &&
		P1=$(ipfs object patch $EMPTY append-data "hello") &&
		P2=$(ipfs object patch $P1 append-data " world") &&
		ipfs object data $P2 >append_data &&
		test_cmp append_data_expected append_data_expected
	'

	test_expect_success "multilayer patch add-link auto-creates directories" '
		echo "hello world" > hwfile &&
		FILE=$(ipfs add -q hwfile) &&
		EMPTY=$(ipfs object new unixfs-dir) &&
		ipfs object patch $EMPTY add-link a/b/c $FILE >multi_patch_add_auto_create_directories
	'

	test_expect_success "multilayer patch add-link auto-create leaf looks good" '
		ipfs cat $(<multi_patch_add_auto_create_directories)/a/b/c >multi_patch_add_auto_create_leaf &&
		test_cmp hwfile multi_patch_add_auto_create_leaf
	'

	test_expect_success "multilayer patch add-link auto-create intermediate looks good" '
		cat <<-\EOF >multi_patch_add_auto_create_intermediate_expected_newline &&
			{
			  "Links": [
			    {
			      "Name": "c",
			      "Hash": "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
			      "Size": 20
			    }
			  ],
			  "Data": ""
			}
		EOF
		printf %s "$(<multi_patch_add_auto_create_intermediate_expected_newline)" >multi_patch_add_auto_create_intermediate_expected &&
		ipfs object get $(<multi_patch_add_auto_create_directories)/a/b >multi_patch_add_auto_create_intermediate &&
		test_cmp multi_patch_add_auto_create_intermediate_expected multi_patch_add_auto_create_intermediate
	'

	test_expect_success "multilayer patch add works" '
		echo "hello world" > hwfile &&
		FILE=$(ipfs add -q hwfile) &&
		EMPTY=$(ipfs object new unixfs-dir) &&
		ONE=$(ipfs object patch $EMPTY add-link b $EMPTY) &&
		TWO=$(ipfs object patch $EMPTY add-link a $ONE) &&
		ipfs object patch $TWO add-link a/b/c $FILE > multi_patch
	'

	test_expect_success "multilayer patch add leaf looks good" '
		ipfs cat $(<multi_patch)/a/b/c > hwfile_out &&
		test_cmp hwfile hwfile_out
	'

	test_expect_success "multilayer patch rm-link works" '
		echo "hello world" > hwfile &&
		FILE=$(ipfs add -q hwfile) &&
		EMPTY=$(ipfs object new unixfs-dir) &&
		ROOT=$(ipfs object patch $EMPTY add-link a/b/c/d $FILE) &&
		ipfs object patch $ROOT rm-link a/b/c >multi_patch_remove
	'

	test_expect_success "multilayer patch rm-link looks good" '
		cat <<-\EOF >multi_patch_remove_expected_newline &&
			{
			  "Links": [
			    {
			      "Name": "c",
			      "Hash": "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
			      "Size": 20
			    }
			  ],
			  "Data": ""
			}
		EOF
		printf %s "$(<multi_patch_remove_expected_newline)" >multi_patch_remove_expected &&
		ipfs object get $(<multi_patch_remove)/a/b > multi_patch_remove_actual &&
		test_cmp hwfile hwfile_out
	'
}

# should work offline
test_object_cmd

# should work online
test_launch_ipfs_daemon
test_object_cmd
test_kill_ipfs_daemon

test_done
