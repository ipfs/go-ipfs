#!/usr/bin/env bash

test_description="Test multibase commands"

. lib/test-lib.sh

# note: all "ipfs multibase" commands should work without requiring a repo

cat <<EOF > bases_expect
      0  identity
0    48  base2
b    98  base32
B    66  base32upper
c    99  base32pad
C    67  base32padupper
f   102  base16
F    70  base16upper
k   107  base36
K    75  base36upper
m   109  base64
M    77  base64pad
t   116  base32hexpad
T    84  base32hexpadupper
u   117  base64url
U    85  base64urlpad
v   118  base32hex
V    86  base32hexupper
z   122  base58btc
Z    90  base58flickr
EOF

# TODO: expose same cmd under multibase?
test_expect_success "multibase list" '
  cut -c 10- bases_expect > expect &&
  ipfs multibase list > actual &&
  test_cmp expect actual
'

test_expect_success "multibase encode works (stdin)" '
  echo -n uaGVsbG8 > expected &&
  echo -n hello | ipfs multibase encode > actual &&
  test_cmp actual expected
'

test_expect_success "multibase encode works (file)" '
  echo -n hello > file &&
  echo -n uaGVsbG8 > expected &&
  ipfs multibase encode ./file > actual &&
  test_cmp actual expected
'

test_expect_success "multibase encode -b (custom base)" '
  echo -n f68656c6c6f > expected &&
  echo -n hello | ipfs multibase encode -b base16 > actual &&
  test_cmp actual expected
'

test_expect_success "multibase decode works (stdin)" '
  echo -n hello > expected &&
  echo -n uaGVsbG8 | ipfs multibase decode > actual &&
  test_cmp actual expected
'

test_expect_success "multibase decode works (file)" '
  echo -n uaGVsbG8 > file &&
  echo -n hello > expected &&
  ipfs multibase decode ./file > actual &&
  test_cmp actual expected
'

test_expect_success "multibase encode+decode roundtrip" '
  echo -n hello > expected &&
  cat expected | ipfs multibase encode -b base64 | ipfs multibase decode > actual &&
  test_cmp actual expected
'

test_expect_success "multibase error on unknown multibase prefix" '
  echo "Error: failed to decode multibase: selected encoding not supported" > expected &&
  echo -n ę-that-should-do-the-trick | ipfs multibase decode 2> actual ;
  test_cmp actual expected
'

test_expect_success "multibase error on a character outside of the base" "
  echo \"Error: failed to decode multibase: encoding/hex: invalid byte: U+007A 'z'\" > expected &&
  echo -n f6c6f6cz | ipfs multibase decode 2> actual ;
  test_cmp actual expected
"

test_done
