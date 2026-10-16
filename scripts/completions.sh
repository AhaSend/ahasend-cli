#!/bin/sh
# Writes the shell completion files that the release archives and packages
# ship into completions/, from the output of `ahasend completion`. It builds
# the CLI for this machine in a temporary folder outside the work tree.
# GoReleaser runs it before the builds.
set -eu

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

CGO_ENABLED=0 go build -trimpath -o "$tmp/ahasend" .
mkdir -p completions
"$tmp/ahasend" completion bash > completions/ahasend.bash
"$tmp/ahasend" completion zsh > completions/_ahasend
"$tmp/ahasend" completion fish > completions/ahasend.fish
