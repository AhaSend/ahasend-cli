#!/bin/sh
# Tests the AUR package that GoReleaser writes into <dist>/aur, generated
# with AHASEND_RELEASES_URL set to a fake release's URL, such as
# http://127.0.0.1:8484. It serves the staged assets of v99.0.0 in
# <release>/v99.0.0 at that URL with the fake-release server:
#
#   test/install/aur.sh <dist> <release> <fake-release>
#
# It runs on Arch Linux with namcap, as a user other than root who can run
# sudo without a password. It builds and installs the package with
# makepkg -si, checks the PKGBUILD and the package with namcap, runs
# ahasend --version, checks the installed files, and removes the package
# with pacman -R. It needs no network.
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <dist> <release> <fake-release>" >&2
	exit 1
fi
dist=$(cd "$1" && pwd)
release=$(cd "$2" && pwd)
server=$(cd "$(dirname "$3")" && pwd)/$(basename "$3")
: "${AHASEND_RELEASES_URL:?set AHASEND_RELEASES_URL to the URL the PKGBUILD downloads from}"

fail() {
	echo "aur: $*" >&2
	exit 1
}

tmp=$(mktemp -d)
server_pid=
trap 'kill "$server_pid" 2> /dev/null || true; rm -rf "$tmp"' EXIT
# A new HOME, so nothing reads or writes the user's own files
export HOME="$tmp/home"
mkdir "$HOME"

"$server" -url "$tmp/url" -latest "$tmp/latest" -addr "${AHASEND_RELEASES_URL#http://}" "$release" 2> "$tmp/server.log" &
server_pid=$!
tries=0
while [ ! -s "$tmp/url" ]; do
	tries=$((tries + 1))
	if [ "$tries" -gt 100 ]; then
		cat "$tmp/server.log" >&2
		fail "the fake release server did not start"
	fi
	sleep 0.1
done

# check_namcap FILE runs namcap on FILE, printing its output, and fails on
# an error; warnings only show in the log
check_namcap() {
	namcap "$1" > "$tmp/namcap" 2>&1 || fail "namcap could not check $1: $(cat "$tmp/namcap")"
	cat "$tmp/namcap"
	if grep -q ' E: ' "$tmp/namcap"; then
		fail "namcap reports errors in $1"
	fi
}

mkdir "$tmp/build"
cp "$dist/aur/ahasend-bin.pkgbuild" "$tmp/build/PKGBUILD"
cd "$tmp/build"

echo "Checking the PKGBUILD"
check_namcap PKGBUILD

echo "Building and installing the package"
makepkg -si --noconfirm

# Arch's default makepkg.conf also makes a debug package,
# ahasend-bin-debug, and installs it too
echo "Checking the package"
set -- ahasend-bin-[0-9]*.pkg.tar*
if [ $# -ne 1 ] || [ ! -f "$1" ]; then
	fail "makepkg did not write one package: $*"
fi
check_namcap "$1"

echo "Checking the installed binary"
ahasend --version > "$tmp/version"
cat "$tmp/version"
sed -n 1p "$tmp/version" | grep -qx 'AhaSend CLI v99.0.0' || fail "ahasend prints the wrong version"

echo "Checking the installed files"
pacman -Ql ahasend-bin
[ -x /usr/bin/ahasend ] || fail "/usr/bin/ahasend is missing"
for file in /usr/share/bash-completion/completions/ahasend \
	/usr/share/zsh/site-functions/_ahasend \
	/usr/share/fish/vendor_completions.d/ahasend.fish \
	/usr/share/licenses/ahasend-bin/LICENSE \
	/usr/share/licenses/ahasend-bin/THIRD_PARTY_NOTICES; do
	[ -s "$file" ] || fail "$file is missing or empty"
done

echo "Removing the package"
packages=ahasend-bin
if pacman -Q ahasend-bin-debug > /dev/null 2>&1; then
	packages="$packages ahasend-bin-debug"
fi
# shellcheck disable=SC2086 # one word per package
sudo pacman -R --noconfirm $packages
[ ! -e /usr/bin/ahasend ] || fail "pacman -R left /usr/bin/ahasend"

echo "The AUR package works"
