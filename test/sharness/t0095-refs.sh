#!/usr/bin/env bash
#
# Copyright (c) 2018 Protocol Labs, Inc
# MIT Licensed; see the LICENSE file in this repository.
#

test_description="Test 'ipfs refs' command"

. lib/test-lib.sh

test_init_ipfs
test_launch_ipfs_daemon_without_network

# This file performs tests with the following directory
# structure.
#
# L0-         _______ A_________
#            /        |     \   \
# L1-      B          C      D   1.txt
#         / \         |      |
# L2-   D   1.txt     B     2.txt
#       |            / \
# L3- 2.txt         D   1.txt
#                   |
# L4-             2.txt
#
# 'ipfs add -r A' output:
#
# added QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v A/1.txt
# added QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v A/B/1.txt
# added QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 A/B/D/2.txt
# added QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v A/C/B/1.txt
# added QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 A/C/B/D/2.txt
# added QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 A/D/2.txt
# added QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS A/B/D
# added QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa A/B
# added QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS A/C/B/D
# added QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa A/C/B
# added QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH A/C
# added QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS A/D
# added QmU6xujRsYzcrkocuR3fhfnkZBB8eyUFFq4WKRGw2aS15h A
#
# 'ipfs refs -r QmU6xujRsYzcrkocuR3fhfnkZBB8eyUFFq4WKRGw2aS15h' sample output
# that shows visit order in a stable go-ipfs version:
#
# QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v - 1.txt
# QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa - B (A/B)
# QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v - 1.txt (A/B/1.txt)
# QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS - D (A/B/D)
# QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 - 2.txt (A/B/D/2.txt)
# QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH - C (A/C)
# QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa - B (A/C/B)
# QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v - 1.txt (A/C/B/1.txt)
# QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS - D (A/C/B/D)
# QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 - 2.txt (A/C/B/D/2.txt)
# QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS - D (A/D)
# QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61 - 2.txt (A/D/2.txt)


refsroot=QmU6xujRsYzcrkocuR3fhfnkZBB8eyUFFq4WKRGw2aS15h

test_expect_success "create and add folders for refs" '
    mkdir -p A/B/D A/C/B/D A/D
    echo "1" > A/1.txt
    echo "1" > A/B/1.txt
    echo "1" > A/C/B/1.txt
    echo "2" > A/B/D/2.txt
    echo "2" > A/C/B/D/2.txt
    echo "2" > A/D/2.txt
    root=$(ipfs add -r -Q A)
    [[ "$root" == "$refsroot" ]]
'

test_refs_output() {
  ARGS=$1
  FILTER=$2

  test_expect_success "ipfs refs $ARGS -r" '
    cat <<EOF | $FILTER > expected.txt
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
EOF

    ipfs refs $ARGS -r $refsroot > refsr.txt
    test_cmp expected.txt refsr.txt
  '

  # Unique is like above but removing duplicates
  test_expect_success "ipfs refs $ARGS -r --unique" '
    cat <<EOF | $FILTER > expected.txt
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH
EOF

    ipfs refs $ARGS -r --unique $refsroot > refsr.txt
    test_cmp expected.txt refsr.txt
  '

  # First level is 1.txt, B, C, D
  test_expect_success "ipfs refs $ARGS" '
    cat <<EOF | $FILTER > expected.txt
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
EOF
    ipfs refs $ARGS $refsroot > refs.txt
    test_cmp expected.txt refs.txt
  '

  # max-depth=0 should return an empty list
  test_expect_success "ipfs refs $ARGS -r --max-depth=0" '
    cat <<EOF > expected.txt
EOF
    ipfs refs $ARGS -r --max-depth=0 $refsroot > refs.txt
    test_cmp expected.txt refs.txt
  '

  # max-depth=1 should be equivalent to running without -r
  test_expect_success "ipfs refs $ARGS -r --max-depth=1" '
    ipfs refs $ARGS -r --max-depth=1 $refsroot > refsr.txt
    ipfs refs $ARGS $refsroot > refs.txt
    test_cmp refsr.txt refs.txt
  '

  # We should see the depth limit engage at level 2
  test_expect_success "ipfs refs $ARGS -r --max-depth=2" '
    cat <<EOF | $FILTER > expected.txt
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
EOF
    ipfs refs $ARGS -r --max-depth=2 $refsroot > refsr.txt
    test_cmp refsr.txt expected.txt
  '

  # Here branch pruning and re-exploration come into place
  # At first it should see D at level 2 and don't go deeper.
  # But then after doing C it will see D at level 1 and go deeper
  # so that it outputs the hash for 2.txt (-q61).
  # We also see that C/B is pruned as it's been shown before.
  #
  # Excerpt from diagram above:
  #
  # L0-         _______ A_________
  #            /        |     \   \
  # L1-      B          C      D   1.txt
  #         / \         |      |
  # L2-   D   1.txt     B     2.txt
  test_expect_success "ipfs refs $ARGS -r --unique --max-depth=2" '
    cat <<EOF | $FILTER > expected.txt
QmdytmR4wULMd3SLo6ePF4s3WcRHWcpnJZ7bHhoj3QB13v
QmNkQvpiyAEtbeLviC7kqfifYoK1GXPcsSxTpP1yS3ykLa
QmSanP5DpxpqfDdS4yekHY1MqrVge47gtxQcp2e2yZ4UwS
QmXXazTjeNCKFnpW1D65vTKsTs8fbgkCWTv8Em4pdK2coH
QmSFxnK675wQ9Kc1uqWKyJUaNxvSc2BP5DbXCD3x93oq61
EOF
    ipfs refs $ARGS -r --unique --max-depth=2 $refsroot > refsr.txt
    test_cmp refsr.txt expected.txt
  '
}

test_refs_output '' 'cat'

test_refs_output '--cid-base=base32' 'ipfs cid base32'

test_kill_ipfs_daemon

test_done
