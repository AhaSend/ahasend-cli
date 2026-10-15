#!/bin/sh
# Copies the release assets of a GoReleaser run into a folder, under the names
# they are published with: checksums.txt and every file it lists. It finds
# each built asset in <dist>/artifacts.json, because GoReleaser keeps the raw
# binaries in a folder per build; a name artifacts.json does not list is an
# extra file, taken from the repository root. Run it from the repository root;
# it needs jq.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <dist> <out>" >&2
	exit 1
fi
dist=$1
out=$2

mkdir -p "$out"
cp "$dist/checksums.txt" "$out/"
awk '{ print $2 }' "$dist/checksums.txt" | while read -r name; do
	path=$(jq -r --arg name "$name" '[.[] | select(.name == $name) | .path] | unique | .[]' "$dist/artifacts.json")
	if [ -z "$path" ]; then
		path=$name
	fi
	if [ "$(echo "$path" | wc -l)" -ne 1 ]; then
		echo "$dist/artifacts.json has more than one file named $name" >&2
		exit 1
	fi
	cp "$path" "$out/$name"
done
