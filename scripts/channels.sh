#!/bin/sh
# Prints the published channels that publish-channel.yml publishes for a
# release tag, one per line: every channel for a stable tag, latest or
# older, and none for a pre-release; see release-kind.sh.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <tag>" >&2
	exit 1
fi

kind=$("$(dirname "$0")/release-kind.sh" "$1")
if [ "$kind" != prerelease ]; then
	echo homebrew
	echo scoop
	echo aur
fi
