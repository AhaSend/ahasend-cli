#!/bin/sh
# Fills in the WinGet manifests in packaging/winget/ for a release tag: the
# version, default-locale and installer manifests of AhaSend.CLI, with the
# URL of each Windows zip and its SHA-256 from <assets-dir>/checksums.txt.
# The URLs start with AHASEND_RELEASES_URL, so a test can point them at a
# fake release. It writes the manifests into <out>, which the WinGet job
# uploads; for the first version a person submits them by hand, in
# manifests/a/AhaSend/CLI/<version>/ of a microsoft/winget-pkgs PR.
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <tag> <assets-dir> <out>" >&2
	exit 1
fi
tag=$1
assets=$2
out=$3

if ! printf '%s\n' "$tag" | grep -Eq '^v[0-9]+\.[0-9]+\.[0-9]+(-[0-9A-Za-z.-]+)?$'; then
	echo "winget-manifest: $tag is not a release tag" >&2
	exit 1
fi
version=${tag#v}
releases_url=${AHASEND_RELEASES_URL:-https://github.com/AhaSend/ahasend-cli/releases}

# sha256_of NAME prints the SHA-256 that checksums.txt gives the asset NAME
sha256_of() {
	sum=$(awk -v name="$1" '$2 == name { print $1 }' "$assets/checksums.txt")
	if [ -z "$sum" ]; then
		echo "winget-manifest: $assets/checksums.txt has no line for $1" >&2
		exit 1
	fi
	echo "$sum"
}
x64_sha256=$(sha256_of "ahasend_${version}_windows_amd64.zip")
arm64_sha256=$(sha256_of "ahasend_${version}_windows_arm64.zip")

# The URL goes into a sed replacement, where & and the | delimiter are special
url=$(printf '%s\n' "$releases_url" | sed 's/[&|\\]/\\&/g')

mkdir -p "$out"
for template in "$(dirname "$0")"/../packaging/winget/*.yaml; do
	sed -e "s|@VERSION@|$version|g" -e "s|@RELEASES_URL@|$url|g" \
		-e "s|@X64_SHA256@|$x64_sha256|g" -e "s|@ARM64_SHA256@|$arm64_sha256|g" \
		"$template" > "$out/$(basename "$template")"
done
echo "Wrote the WinGet manifests of AhaSend.CLI $version into $out"
