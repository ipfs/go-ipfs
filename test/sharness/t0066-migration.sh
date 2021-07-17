#!/usr/bin/env bash
#
# Copyright (c) 2016 Jeromy Johnson
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test migrations auto update prompt"

. lib/test-lib.sh

test_init_ipfs

MIGRATION_START=7
IPFS_REPO_VER=$(<.ipfs/version)

# Generate mock migration binaries
gen_mock_migrations() {
  mkdir bin
  i=$((MIGRATION_START))
  until [ $i -ge $IPFS_REPO_VER ]
  do
    j=$((i+1))
    echo "#!/bin/bash" > bin/fs-repo-${i}-to-${j}
    echo "echo fake applying ${i}-to-${j} repo migration" >> bin/fs-repo-${i}-to-${j}
    chmod +x bin/fs-repo-${i}-to-${j}
    ((i++))
  done
}

# Check for expected output from each migration
check_migration_output() {
  out_file="$1"
  i=$((MIGRATION_START))
  until [ $i -ge $IPFS_REPO_VER ]
  do
    j=$((i+1))
    grep "applying ${i}-to-${j} repo migration" "$out_file" > /dev/null
    ((i++))
  done
}

# Create fake migration binaries instead of letting ipfs download from network
# To test downloading and running actual binaries, comment out this test.
test_expect_success "setup mock migrations" '
  gen_mock_migrations &&
  find bin -name "fs-repo-*-to-*" | wc -l > mock_count &&
  echo $((IPFS_REPO_VER-MIGRATION_START)) > expect_mock_count &&
  export PATH="$(pwd)/bin":$PATH &&
  test_cmp mock_count expect_mock_count
'

test_expect_success "manually reset repo version to $MIGRATION_START" '
  echo "$MIGRATION_START" > "$IPFS_PATH"/version
'

test_expect_success "ipfs daemon --migrate=false fails" '
  test_expect_code 1 ipfs daemon --migrate=false > false_out
'

test_expect_success "output looks good" '
  grep "Please get fs-repo-migrations from https://dist.ipfs.io" false_out
'

# The migrations will succeed, but the daemon will still exit with 1 because
# the fake migrations do not update the repo version number.
#
# If run with real migrations, the daemon continues running and must be killed.
test_expect_success "ipfs daemon --migrate=true runs migration" '
  test_expect_code 1 ipfs daemon --migrate=true > true_out
'

test_expect_success "output looks good" '
  check_migration_output true_out &&
  grep "Success: fs-repo migrated to version $IPFS_REPO_VER" true_out > /dev/null
'

test_expect_success "'ipfs daemon' prompts to auto migrate" '
  test_expect_code 1 ipfs daemon > daemon_out 2> daemon_err
'

test_expect_success "output looks good" '
  grep "Found outdated fs-repo" daemon_out > /dev/null &&
  grep "Run migrations now?" daemon_out > /dev/null &&
  grep "Please get fs-repo-migrations from https://dist.ipfs.io" daemon_out > /dev/null
'

test_expect_success "ipfs repo migrate succeed" '
  test_expect_code 0 ipfs repo migrate > migrate_out
'

test_expect_success "output looks good" '
  grep "Found outdated fs-repo, starting migration." migrate_out > /dev/null &&
  grep "Success: fs-repo migrated to version $IPFS_REPO_VER" true_out > /dev/null
'

test_expect_success "manually reset repo version to 11" '
  echo "$IPFS_REPO_VER" > "$IPFS_PATH"/version
'

test_expect_success "detect repo does not need migration" '
  test_expect_code 0 ipfs repo migrate > migrate_out
'

test_expect_success "output looks good" '
  grep "Repo does not require migration" migrate_out > /dev/null
'
cat migrate_out

test_done
