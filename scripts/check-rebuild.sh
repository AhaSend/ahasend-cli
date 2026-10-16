#!/bin/sh
# Checks that a rebuild of a release made the same assets as the release: its
# checksums.txt must be the same as the release's. publish-channel.yml runs
# it before it publishes the files of the rebuild, so they always describe
# the release's own assets.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <rebuilt checksums.txt> <release checksums.txt>" >&2
	exit 1
fi

if [ ! -s "$1" ] || [ ! -s "$2" ]; then
	echo "check-rebuild: $1 or $2 is missing or empty" >&2
	exit 1
fi
if ! diff -u "$2" "$1" >&2; then
	echo "check-rebuild: the rebuilt assets differ from the release's; nothing is published" >&2
	exit 1
fi
echo "The rebuilt assets are the same as the release's"
