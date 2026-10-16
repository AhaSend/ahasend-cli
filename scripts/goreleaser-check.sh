#!/bin/sh
# Runs `goreleaser check` on .goreleaser.yaml (or the given file), with the
# GoReleaser version of the Makefile and the Go toolchain of go.mod. It passes
# when the check passes, or when brews, which the Homebrew formula needs, is
# the only deprecated property it reports.
set -eu

root=$(cd "$(dirname "$0")/.." && pwd)
goreleaser_version=$(sed -n 's/^GORELEASER_VERSION := //p' "$root/Makefile")
go_toolchain=$(sed -n 's/^toolchain //p' "$root/go.mod")

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# Installed rather than run with go run, which hides the exit code
GOTOOLCHAIN=$go_toolchain GOBIN=$tmp go install "github.com/goreleaser/goreleaser/v2@$goreleaser_version"
status=0
"$tmp/goreleaser" check "$@" > "$tmp/output" 2>&1 || status=$?
cat "$tmp/output"
if [ "$status" -eq 0 ]; then
	exit 0
fi
if [ "$status" -ne 2 ]; then
	exit "$status"
fi

# Exit code 2: the configuration is valid but uses deprecated properties.
# With CI set, GoReleaser colours its output even in a file, so the colour
# codes are removed first.
esc=$(printf '\033')
deprecated=$(sed "s/$esc\[[0-9;]*m//g" "$tmp/output" |
	sed -n 's/.*DEPRECATED: *\([^ ]*\) .*/\1/p' | sort -u)
if [ "$deprecated" != brews ]; then
	echo "goreleaser-check: deprecated properties other than brews:" "$(echo "$deprecated" | tr '\n' ' ')" >&2
	exit 1
fi
