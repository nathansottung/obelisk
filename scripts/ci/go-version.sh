#!/usr/bin/env bash
# Prints version=<X.Y.Z> from go.mod's `toolchain goX.Y.Z` line, for $GITHUB_OUTPUT.
# Fails if the line is missing or the Dockerfile's GO_VERSION default differs, so
# go.mod stays the one source of truth for the Go version.
set -euo pipefail
cd "$(dirname "$0")/../.."
v=$(sed -n 's/^toolchain go\([0-9][0-9]*\.[0-9][0-9]*\.[0-9][0-9]*\)\r\{0,1\}$/\1/p' go.mod)
d=$(sed -n 's/^ARG GO_VERSION=\([0-9.]*\)\r\{0,1\}$/\1/p' Dockerfile)
if [ -z "$v" ]; then echo "go.mod has no 'toolchain goX.Y.Z' line" >&2; exit 1; fi
if [ "$v" != "$d" ]; then echo "Dockerfile GO_VERSION '$d' does not match go.mod toolchain '$v'" >&2; exit 1; fi
echo "version=$v"
