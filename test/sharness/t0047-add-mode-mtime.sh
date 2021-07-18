#!/usr/bin/env bash

test_description="Test storing and retrieving mode and mtime"

. lib/test-lib.sh

test_init_ipfs

TESTFILE=mountdir/testfile.txt

PRESERVE_MTIME=1604320482
PRESERVE_MODE=0640
HASH_PRESERVE_MODE=QmQLgxypSNGNFTuUPGCecq6dDEjb6hNB5xSyVmP3cEuNtq
HASH_PRESERVE_MTIME=QmbfPiiFH3mM9qCCsCL1BdDfDBxeCrVXfiiSEUYdjPzeQf
HASH_PRESERVE_MODE_AND_MTIME=QmT858Bjy215QhtJKBiEueLZq2VDoiiJ6dDP2RNAkPMT7f

CUSTOM_MTIME=1603539720
CUSTOM_MTIME_NSECS=54321
CUSTOM_MODE=0764
HASH_CUSTOM_MODE=QmchD3BN8TQ3RW6jPLxSaNkqvfuj7syKhzTRmL4EpyY1Nz
HASH_CUSTOM_MTIME=QmS7vmVxe9YgbWm2CjEKBTYnhhrUjy6cbnDTqe4tkPhu32
HASH_CUSTOM_MTIME_NSECS=QmaKH8H5rXBUBCX4vdxi7ktGQEL7wejV7L9rX2qpZjwncz
HASH_CUSTOM_MODE_AND_MTIME=QmUkxrtBA8tPjwCYz1HrsoRfDz6NgKut3asVeHVQNH4C8L

test_expect_success "can preserve file mode" '
  touch "$TESTFILE" &&
  chmod $PRESERVE_MODE "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --preserve-mode "$TESTFILE") &&
  test "$HASH_PRESERVE_MODE" = "$HASH"
'

test_expect_success "can preserve file modification time" '
  touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --preserve-mtime "$TESTFILE") &&
  test "$HASH_PRESERVE_MTIME" = "$HASH"
'

test_expect_success "can set file mode" '
  touch "$TESTFILE" &&
  chmod 0600 "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --mode=$CUSTOM_MODE "$TESTFILE") &&
  test "$HASH_CUSTOM_MODE" = "$HASH"
'

test_expect_success "can set file mtime" '
  touch -m -t 202011021234.42 "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --mtime=$CUSTOM_MTIME "$TESTFILE") &&
  test "$HASH_CUSTOM_MTIME" = "$HASH"
'

test_expect_success "can set file mtime nanoseconds" '
  touch -m -t 202011021234.42 "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
  test "$HASH_CUSTOM_MTIME_NSECS" = "$HASH"
'

test_expect_success "can preserve file mode and modification time" '
  touch -m -d @$PRESERVE_MTIME "$TESTFILE" &&
  chmod $PRESERVE_MODE "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --preserve-mode --preserve-mtime "$TESTFILE") &&
  test "$HASH_PRESERVE_MODE_AND_MTIME" = "$HASH"
'

test_expect_success "can set file mode and mtime" '
  touch -m -t 202011021234.42 "$TESTFILE" &&
  chmod 0600 "$TESTFILE" &&
  HASH=$(ipfs add -q --hash=sha2-256 --mode=$CUSTOM_MODE --mtime=$CUSTOM_MTIME --mtime-nsecs=$CUSTOM_MTIME_NSECS "$TESTFILE") &&
  test "$HASH_CUSTOM_MODE_AND_MTIME" = "$HASH"
'

test_expect_success "can get preserved mode and mtime" '
  OUTFILE="mountdir/$HASH_PRESERVE_MODE_AND_MTIME" &&
  ipfs get -o "$OUTFILE" $HASH_PRESERVE_MODE_AND_MTIME &&
  test "$PRESERVE_MODE:$PRESERVE_MTIME" = "$(stat -c "0%a:%Y" "$OUTFILE")"
'

test_expect_success "can get custom mode and mtime" '
  OUTFILE="mountdir/$HASH_CUSTOM_MODE_AND_MTIME" &&
  ipfs get -o "$OUTFILE" $HASH_CUSTOM_MODE_AND_MTIME &&
  TIMESTAMP=$(date +%s%N --date="$(stat -c "%y" $OUTFILE)") &&
  MODETIME=$(stat -c "0%a:$TIMESTAMP" "$OUTFILE") &&
  printf -v EXPECTED "$CUSTOM_MODE:$CUSTOM_MTIME%09d" $CUSTOM_MTIME_NSECS &&
  test "$EXPECTED" = "$MODETIME"
'

test_done
