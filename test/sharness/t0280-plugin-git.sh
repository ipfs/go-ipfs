#!/usr/bin/env bash
#
# Copyright (c) 2017 Jakub Sztandera
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test git plugin"

. lib/test-lib.sh

test_init_ipfs

# from https://github.com/ipfs/go-ipld-git/blob/master/make-test-repo.sh
test_expect_success "prepare test data" '
  tar xzf ../t0280-plugin-git-data/git.tar.gz
'

test_dag_git() {
  test_expect_success "add objects via dag put" '
    find objects -type f -exec ipfs dag put --format=git --input-enc=zlib {} \; -exec echo \; > hashes
  '

  test_expect_failure "successfully get added objects" '
    cat hashes | xargs -I {} ipfs dag get -- {} > /dev/null
  '

  test_expect_failure "path traversals work" '
    echo \"YmxvYiA3ACcsLnB5Zgo=\" > file1 &&
    ipfs dag get baf4bcfhzi72pcj5cc4ocz7igcduubuu7aa3cddi/object/parents/0/tree/dir2/hash/f3/hash > out1
  '

  test_expect_failure "outputs look correct" '
    test_cmp file1 out1
  '
}

# should work offline
#test_dag_git

# should work online
test_launch_ipfs_daemon
test_dag_git
test_kill_ipfs_daemon

test_done
