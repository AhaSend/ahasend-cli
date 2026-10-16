#!/bin/sh
# Prints the kind of a release tag: prerelease for vX.Y.Z-<suffix>, latest
# for a stable vX.Y.Z not lower than any other stable tag of the repository,
# and older for a stable tag lower than another one. It fails for any other
# tag. It reads the tags of the repository in the current directory.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <tag>" >&2
	exit 1
fi
tag=$1
stable='^v[0-9]+\.[0-9]+\.[0-9]+$'

if echo "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+-[0-9A-Za-z.-]+$'; then
	echo prerelease
	exit 0
fi
if ! echo "$tag" | grep -Eq "$stable"; then
	echo "$tag is not a release tag: use vX.Y.Z or vX.Y.Z-<suffix>" >&2
	exit 1
fi

tags=$(git tag --list 'v*')
highest=$(printf '%s\n%s\n' "$tags" "$tag" | grep -E "$stable" | sort -V | tail -n 1)
if [ "$highest" = "$tag" ]; then
	echo latest
else
	echo older
fi
