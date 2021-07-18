#!/usr/bin/env bash

test_description="Test config command"

. lib/test-lib.sh

# we use a function so that we can run it both offline + online
test_config_cmd_set() {

  # flags (like --bool in "ipfs config --bool")
  cfg_flags="" # unset in case.
  test "$#" = 3 && { cfg_flags=$1; shift; }

  cfg_key=$1
  cfg_val=$2
  test_expect_success "ipfs config succeeds" '
    ipfs config $cfg_flags "$cfg_key" "$cfg_val"
  '

  test_expect_success "ipfs config output looks good" '
    echo "$cfg_val" >expected &&
    ipfs config "$cfg_key" >actual &&
    test_cmp expected actual
  '

  # also test our lib function. it should work too.
  cfg_key="Lib.$cfg_key"
  test_expect_success "test_config_set succeeds" '
    test_config_set $cfg_flags "$cfg_key" "$cfg_val"
  '

  test_expect_success "test_config_set value looks good" '
    echo "$cfg_val" >expected &&
    ipfs config "$cfg_key" >actual &&
    test_cmp expected actual
  '
}

# this is a bit brittle. the problem is we need to test
# with something that will be forced to unmarshal as a struct.
# (i.e. just setting 'ipfs config --json foo "[1, 2, 3]"') may
# set it as astring instead of proper json. We leverage the
# unmarshalling that has to happen.
CONFIG_SET_JSON_TEST='{
  "MDNS": {
    "Enabled": true,
    "Interval": 10
  }
}'

test_profile_apply_revert() {
  profile=$1
  inverse_profile=$2

  test_expect_success "save expected config" '
    ipfs config show >expected
  '

  test_expect_success "'ipfs config profile apply ${profile}' works" '
    ipfs config profile apply '${profile}'
  '

  test_expect_success "profile ${profile} changed something" '
    ipfs config show >actual &&
    test_must_fail test_cmp expected actual
  '

  test_expect_success "'ipfs config profile apply ${inverse_profile}' works" '
    ipfs config profile apply '${inverse_profile}'
  '

  test_expect_success "config is back to previous state after ${inverse_profile} was applied" '
    ipfs config show >actual &&
    test_cmp expected actual
  '
}

test_profile_apply_dry_run_not_alter() {
  profile=$1

  test_expect_success "'ipfs config profile apply ${profile} --dry-run' doesn't alter config" '
    cat "$IPFS_PATH/config" >expected &&
    ipfs config profile apply '${profile}' --dry-run &&
    cat "$IPFS_PATH/config" >actual &&
    test_cmp expected actual
  '
}

test_config_cmd() {
  test_config_cmd_set "beep" "boop"
  test_config_cmd_set "beep1" "boop2"
  test_config_cmd_set "beep1" "boop2"
  test_config_cmd_set "--bool" "beep2" "true"
  test_config_cmd_set "--bool" "beep2" "false"
  test_config_cmd_set "--json" "beep3" "true"
  test_config_cmd_set "--json" "beep3" "false"
  test_config_cmd_set "--json" "Discovery" "$CONFIG_SET_JSON_TEST"
  test_config_cmd_set "--json" "deep-not-defined.prop" "true"
  test_config_cmd_set "--json" "deep-null" "null"
  test_config_cmd_set "--json" "deep-null.prop" "true"

  test_expect_success "'ipfs config show' works" '
    ipfs config show >actual
  '

  test_expect_success "'ipfs config show' output looks good" '
    grep "\"beep\": \"boop\"," actual &&
    grep "\"beep1\": \"boop2\"," actual &&
    grep "\"beep2\": false," actual &&
    grep "\"beep3\": false," actual
  '

  test_expect_success "setup for config replace test" '
    cp "$IPFS_PATH/config" newconfig.json &&
    sed -i"~" -e /PrivKey/d -e s/10GB/11GB/ newconfig.json &&
    sed -i"~" -e '"'"'/PeerID/ {'"'"' -e '"'"' s/,$// '"'"' -e '"'"' } '"'"' newconfig.json
  '

  test_expect_success "run 'ipfs config replace'" '
  ipfs config replace - < newconfig.json
  '

  test_expect_success "check resulting config after 'ipfs config replace'" '
    sed -e /PrivKey/d "$IPFS_PATH/config" > replconfig.json &&
    sed -i"~" -e '"'"'/PeerID/ {'"'"' -e '"'"' s/,$// '"'"' -e '"'"' } '"'"' replconfig.json &&
    test_cmp replconfig.json newconfig.json
  '

  # Dynamic keys with dot in their names
  test_config_cmd_set "--json" "Gateway.PublicGateways[\"some.example.com\"].UseSubdomains" "true"
  test_expect_success "'ipfs config show' after Foo[\"bar.buzz\"] returns a valid JSON" '
    ipfs config show | jq -e > /dev/null 2>&1
  '
  # TODO:  ipfs config show | jq -e > /dev/null 2>&1
  test_expect_success "'ipfs config' after Foo[\"bar.buzz\"] shows updated value" '
    ipfs config Gateway.PublicGateways[\"some.example.com\"].UseSubdomains > bool_out &&
    grep true bool_out
  '

  # SECURITY
  # Those tests are here to prevent exposing the PrivKey on the network

  test_expect_success "'ipfs config Identity' fails" '
    test_expect_code 1 ipfs config Identity 2> ident_out
  '

  test_expect_success "output looks good" '
    echo "Error: cannot show or change private key through API" > ident_exp &&
    test_cmp ident_exp ident_out
  '

  test_expect_success "'ipfs config Identity.PrivKey' fails" '
    test_expect_code 1 ipfs config Identity.PrivKey 2> ident_out
  '

  test_expect_success "output looks good" '
    test_cmp ident_exp ident_out
  '

  test_expect_success "lower cased PrivKey" '
    sed -i"~" -e '\''s/PrivKey/privkey/'\'' "$IPFS_PATH/config" &&
    test_expect_code 1 ipfs config Identity.privkey 2> ident_out
  '

  test_expect_success "output looks good" '
    test_cmp ident_exp ident_out
  '

  test_expect_success "fix it back" '
    sed -i"~" -e '\''s/privkey/PrivKey/'\'' "$IPFS_PATH/config"
  '

  test_expect_success "'ipfs config show' doesn't include privkey" '
    ipfs config show > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  test_expect_success "'ipfs config replace' injects privkey back" '
    ipfs config replace show_config &&
    grep "\"PrivKey\":" "$IPFS_PATH/config" | grep -e ": \".\+\"" >/dev/null
  '

  test_expect_success "'ipfs config replace' with privkey errors out" '
    cp "$IPFS_PATH/config" real_config &&
    test_expect_code 1 ipfs config replace - < real_config 2> replace_out
  '

  test_expect_success "output looks good" '
    echo "Error: setting private key with API is not supported" > replace_expected
    test_cmp replace_out replace_expected
  '

  test_expect_success "'ipfs config replace' with lower case privkey errors out" '
    cp "$IPFS_PATH/config" real_config &&
    sed -i -e '\''s/PrivKey/privkey/'\'' real_config &&
    test_expect_code 1 ipfs config replace - < real_config 2> replace_out
  '

  test_expect_success "output looks good" '
    echo "Error: setting private key with API is not supported" > replace_expected
    test_cmp replace_out replace_expected
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 1
  '

  test_expect_success "copy ipfs config" '
    cp "$IPFS_PATH/config" before_patch
  '

  test_expect_success "'ipfs config profile apply server' works" '
    ipfs config profile apply server
  '

  test_expect_success "backup was created and looks good" '
    test_cmp "$(find "$IPFS_PATH" -name "config-*")" before_patch
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good with server profile" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 18
  '

  test_expect_success "'ipfs config profile apply local-discovery' works" '
    ipfs config profile apply local-discovery
  '

  test_expect_success "'ipfs config Swarm.AddrFilters' looks good with applied local-discovery profile" '
    ipfs config Swarm.AddrFilters > actual_config &&
    test $(cat actual_config | wc -l) = 1
  '

  test_profile_apply_revert server local-discovery

  # tests above mess with values this profile changes, need to do that before testing test profile
  test_expect_success "ensure test profile is applied fully" '
    ipfs config profile apply test
  '

  # need to do this in reverse as the test profile is already applied in sharness
  test_profile_apply_revert default-networking test

  test_profile_apply_dry_run_not_alter server

  test_profile_apply_dry_run_not_alter local-discovery

  test_profile_apply_dry_run_not_alter test

  test_expect_success "'ipfs config profile apply local-discovery --dry-run' looks good with different profile info" '
    ipfs config profile apply local-discovery --dry-run > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply server --dry-run' looks good with same profile info" '
    ipfs config profile apply server --dry-run > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 1
  '

  test_expect_success "'ipfs config profile apply server' looks good with same profile info" '
    ipfs config profile apply server > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 1
  '

  test_expect_success "'ipfs config profile apply local-discovery' looks good with different profile info" '
    ipfs config profile apply local-discovery > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply test' looks good with different profile info" '
    ipfs config profile apply test > diff_info &&
    test `grep "DisableNatPortMap" diff_info | wc -l` = 2
  '

  test_expect_success "'ipfs config profile apply test --dry-run' doesn't include privkey" '
    ipfs config profile apply test --dry-run > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  test_expect_success "'ipfs config profile apply test' doesn't include privkey" '
    ipfs config profile apply test > show_config &&
    test_expect_code 1 grep PrivKey show_config
  '

  # won't work as it changes datastore definition, which makes ipfs not launch
  # without converting first
  # test_profile_apply_revert badgerds

  test_expect_success "cleanup config backups" '
    find "$IPFS_PATH" -name "config-*" -exec rm {} \;
  '
}

test_init_ipfs

# should work offline
test_config_cmd

# should work online
test_launch_ipfs_daemon
test_config_cmd
test_kill_ipfs_daemon


test_done
