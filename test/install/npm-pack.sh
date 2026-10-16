#!/bin/sh
# Packs the seven npm packages of a fake release as the npm job of
# publish-channel.yml does, and checks them:
#
#   test/install/npm-pack.sh <release> <fake-release> <out>
#
# It serves the staged assets of v99.0.0 in <release>/v99.0.0 with the
# fake-release server, downloads them with scripts/fetch-release.sh, fills
# in the packages with scripts/npm-packages.mjs and packs each into <out>
# with npm pack. Then it checks every package.json and what every tarball
# holds. test/install/npm.mjs installs the tarballs. It runs on macOS and
# Linux and needs node, npm and jq.
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <release> <fake-release> <out>" >&2
	exit 1
fi
release=$(cd "$1" && pwd)
server=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
mkdir -p "$3"
out=$(cd "$3" && pwd)
cd "$(dirname "$0")/../.."
# shellcheck source=test/release/lib.sh
. test/release/lib.sh

"$server" -url "$tmp/url" -latest "$tmp/latest" "$release" 2> "$tmp/server.log" &
server_pid=$!
trap 'kill "$server_pid" 2> /dev/null || true; rm -rf "$tmp"' EXIT
tries=0
while [ ! -s "$tmp/url" ]; do
	tries=$((tries + 1))
	if [ "$tries" -gt 100 ]; then
		echo "the fake release server did not start:" >&2
		cat "$tmp/server.log" >&2
		exit 1
	fi
	sleep 0.1
done

echo "Packing the npm packages as the npm job does"
AHASEND_RELEASES_URL=$(cat "$tmp/url") scripts/fetch-release.sh v99.0.0 "$tmp/release"
scripts/npm-packages.mjs v99.0.0 "$tmp/release" "$tmp/npm"
for package in "$tmp"/npm/*/; do
	npm pack "$package" --pack-destination "$out" > /dev/null
done

expect_output "there are seven packages" 7 sh -c "ls '$tmp/npm' | wc -l | tr -d ' '"
expect_output "there are seven tarballs" 7 sh -c "ls '$out'/*.tgz | wc -l | tr -d ' '"
# The helpers of lib.sh use name and dir, so the loop uses other names
for pkg_dir in "$tmp"/npm/*/; do
	pkg=$(basename "$pkg_dir")
	json=$pkg_dir/package.json
	tarball=$out/ahasend-$pkg-99.0.0.tgz
	case $pkg in
	cli) binary=bin/ahasend.js ;;
	cli-win32-*) binary=bin/ahasend.exe ;;
	*) binary=bin/ahasend ;;
	esac
	expect_output "$pkg: the version is 99.0.0" 99.0.0 jq -r .version "$json"
	expect_output "$pkg: repository.url is the repository" git+https://github.com/AhaSend/ahasend-cli.git \
		jq -r .repository.url "$json"
	expect_output "$pkg: the licence is Apache-2.0" Apache-2.0 jq -r .license "$json"
	expect_output "$pkg: files lists the binary and the licence files" "$binary LICENSE THIRD_PARTY_NOTICES" \
		jq -r '.files | join(" ")' "$json"
	tar -tvzf "$tarball" > "$tmp/list"
	for file in LICENSE THIRD_PARTY_NOTICES "$binary"; do
		expect_success "$pkg: the tarball holds $file" grep -q " package/$file\$" "$tmp/list"
	done
	case $pkg in
	cli-win32-*) ;;
	*)
		expect_success "$pkg: $binary has mode 755" grep -q "^-rwxr-xr-x .* package/$binary\$" "$tmp/list"
		;;
	esac
done
expect_output "the wrapper pins the six platform packages to its version" \
	"@ahasend/cli-darwin-arm64@99.0.0 @ahasend/cli-darwin-x64@99.0.0 @ahasend/cli-linux-arm64@99.0.0 @ahasend/cli-linux-x64@99.0.0 @ahasend/cli-win32-arm64@99.0.0 @ahasend/cli-win32-x64@99.0.0" \
	jq -r '.optionalDependencies | to_entries | map("\(.key)@\(.value)") | join(" ")' "$tmp/npm/cli/package.json"
expect_output "the wrapper needs Node 18" ">=18" jq -r .engines.node "$tmp/npm/cli/package.json"
expect_output "the wrapper has no install scripts" null jq -r .scripts "$tmp/npm/cli/package.json"
expect_output "the platform packages have os, cpu and preferUnplugged and no libc" \
	"darwin arm64 true null
darwin x64 true null
linux arm64 true null
linux x64 true null
win32 arm64 true null
win32 x64 true null" \
	sh -c "for json in '$tmp'/npm/cli-*/package.json; do jq -r '\"\(.os | join(\",\")) \(.cpu | join(\",\")) \(.preferUnplugged) \(.libc)\"' \"\$json\"; done"

finish
echo "The npm packages are packed in $out"
