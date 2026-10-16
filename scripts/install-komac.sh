#!/bin/sh
# Installs Komac, which opens the WinGet PRs, into a folder: it downloads the
# linux x64 release archive of KOMAC_VERSION, checks it against KOMAC_SHA256
# and unpacks the komac binary. Komac runs with WINGET_TOKEN, so only the
# archive whose SHA-256 the repository holds may run. The workflows set both
# variables to the Makefile's values, which make check-versions checks.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <dir>" >&2
	exit 1
fi
dir=$1
: "${KOMAC_VERSION:?set KOMAC_VERSION to the KOMAC_VERSION of the Makefile}"
: "${KOMAC_SHA256:?set KOMAC_SHA256 to the KOMAC_SHA256 of the Makefile}"

version=${KOMAC_VERSION#v}
archive=komac-$version-x86_64-unknown-linux-gnu.tar.gz
mkdir -p "$dir"
curl -fsSL --retry 3 -o "$dir/$archive" \
	"https://github.com/russellbanks/Komac/releases/download/v$version/$archive"
actual=$(sha256sum "$dir/$archive" | awk '{ print $1 }')
if [ "$actual" != "$KOMAC_SHA256" ]; then
	rm -f "$dir/$archive"
	echo "install-komac: the SHA-256 of $archive is $actual, not KOMAC_SHA256 ($KOMAC_SHA256)" >&2
	exit 1
fi
tar -xzf "$dir/$archive" -C "$dir" komac
rm -f "$dir/$archive"
echo "Installed $("$dir/komac" --version) into $dir"
