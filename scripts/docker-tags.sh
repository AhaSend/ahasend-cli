#!/bin/sh
# Prints the Docker image tags of a release tag, one per line: the tag, the
# version without the v, and latest for the newest stable tag only; see
# release-kind.sh.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <tag>" >&2
	exit 1
fi

kind=$("$(dirname "$0")/release-kind.sh" "$1")
echo "$1"
echo "${1#v}"
if [ "$kind" = latest ]; then
	echo latest
fi
