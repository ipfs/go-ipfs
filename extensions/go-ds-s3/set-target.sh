#!/bin/bash

GOCC="${GOCC:-go}"

set -eo pipefail

GOPATH="$($GOCC env GOPATH)"
VERSION="$1"
PKG=github.com/ipfs/go-ipfs

if [[ "$VERSION" == /* ]]; then
    # Build against a local repo
    MODFILE="$VERSION/go.mod"
    $GOCC mod edit -replace "github.com/ipfs/go-ipfs=$VERSION"
else
    $GOCC mod edit -dropreplace=github.com/ipfs/go-ipfs
    # Resolve the exact version/package name
    MODFILE="$(go list -f '{{.GoMod}}' -m "$PKG@$VERSION")"
    resolvedver="$(go list -f '{{.Version}}' -m "$PKG@$VERSION")"

    # Update to that version.
    $GOCC get $PKG@$resolvedver
fi

TMP="$(mktemp -d)"
trap "$(printf 'rm -rf "%q"' "$TMP")" EXIT

(
    cd "$TMP"
    cp "$MODFILE" "go.mod"
    go list -mod=mod -f '-require={{.Path}}@{{.Version}}{{if .Replace}} -replace={{.Path}}@{{.Version}}={{.Replace}}{{end}}' -m all | tail -n+2  > args
)

$GOCC mod edit $(cat "$TMP/args")
$GOCC mod tidy
