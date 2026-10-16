#!/bin/sh
# Fills in the Chocolatey package in packaging/chocolatey/ for a release tag,
# from the release assets in <assets-dir>, and writes it into <out>, ready for
# `choco pack <out>/ahasend.nuspec`:
#
#   ahasend.nuspec                 the nuspec, with the version
#   tools/ahasend.exe              the asset ahasend-windows-amd64.exe
#   tools/LICENSE.txt              the asset LICENSE
#   tools/THIRD_PARTY_NOTICES.txt  the asset THIRD_PARTY_NOTICES
#   tools/VERIFICATION.txt         the exe's release URL and its SHA-256 from
#                                  checksums.txt, for the moderators
#
# Chocolatey makes the ahasend shim for the exe, so the package needs no
# install script. The URLs start with AHASEND_RELEASES_URL, so a test can
# point them at a fake release.
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <tag> <assets-dir> <out>" >&2
	exit 1
fi
tag=$1
assets=$2
out=$3

if ! printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
	echo "choco-package: $tag is not a release tag" >&2
	exit 1
fi
version=${tag#v}
releases_url=${AHASEND_RELEASES_URL:-https://github.com/AhaSend/ahasend-cli/releases}
exe=ahasend-windows-amd64.exe

sha256=$(awk -v name="$exe" '$2 == name { print $1 }' "$assets/checksums.txt")
if [ -z "$sha256" ]; then
	echo "choco-package: $assets/checksums.txt has no line for $exe" >&2
	exit 1
fi
# VERIFICATION.txt gives this SHA-256 for the embedded exe
actual=$(sha256sum "$assets/$exe" | awk '{ print $1 }')
if [ "$actual" != "$sha256" ]; then
	echo "choco-package: $assets/$exe does not match checksums.txt" >&2
	exit 1
fi

# The URL goes into a sed replacement, where & and the | delimiter are special
url=$(printf '%s\n' "$releases_url" | sed 's/[&|\\]/\\&/g')
fill() {
	sed -e "s|@VERSION@|$version|g" -e "s|@RELEASES_URL@|$url|g" -e "s|@SHA256@|$sha256|g" "$1"
}

templates=$(dirname "$0")/../packaging/chocolatey
mkdir -p "$out/tools"
fill "$templates/ahasend.nuspec" > "$out/ahasend.nuspec"
fill "$templates/VERIFICATION.txt" > "$out/tools/VERIFICATION.txt"
cp "$assets/$exe" "$out/tools/ahasend.exe"
cp "$assets/LICENSE" "$out/tools/LICENSE.txt"
cp "$assets/THIRD_PARTY_NOTICES" "$out/tools/THIRD_PARTY_NOTICES.txt"
echo "Wrote the Chocolatey package of ahasend $version into $out"
