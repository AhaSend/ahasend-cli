#!/bin/sh
# Tests the Homebrew formula that GoReleaser writes into
# <dist>/homebrew/Formula/ahasend.rb, generated with AHASEND_RELEASES_URL set
# to a fake release's URL, such as http://127.0.0.1:8484. It serves the
# staged assets of v99.0.0 in <release>/v99.0.0 at that URL with the
# fake-release server:
#
#   test/install/homebrew.sh <dist> <release> <fake-release>
#
# It puts the formula in a local tap, ahasend/tap, installs it by its full
# name, runs brew test and brew audit --strict, checks ahasend --version and
# the installed files, and uninstalls it. It runs on macOS and Linux with
# brew on PATH.
set -eu

if [ $# -ne 3 ]; then
	echo "usage: $0 <dist> <release> <fake-release>" >&2
	exit 1
fi
dist=$(cd "$1" && pwd)
release=$(cd "$2" && pwd)
server=$(cd "$(dirname "$3")" && pwd)/$(basename "$3")
: "${AHASEND_RELEASES_URL:?set AHASEND_RELEASES_URL to the URL the formula downloads from}"
export HOMEBREW_NO_AUTO_UPDATE=1 HOMEBREW_NO_ANALYTICS=1 HOMEBREW_NO_ENV_HINTS=1

fail() {
	echo "homebrew: $*" >&2
	exit 1
}

tmp=$(mktemp -d)
server_pid=
tapped=
# Removes only what this test made: the formula and the tap it created
cleanup() {
	kill "$server_pid" 2> /dev/null || true
	if [ -n "$tapped" ]; then
		brew uninstall --force ahasend/tap/ahasend > /dev/null 2>&1 || true
		brew untap ahasend/tap > /dev/null 2>&1 || true
	fi
	rm -rf "$tmp"
}
trap cleanup EXIT
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

echo "Installing the formula from a local tap"
if [ -d "$(brew --repository)/Library/Taps/ahasend/homebrew-tap" ]; then
	fail "ahasend/tap is already tapped; run this test where it is not"
fi
brew tap-new --no-git ahasend/tap
tapped=1
cp "$dist/homebrew/Formula/ahasend.rb" "$(brew --repository ahasend/tap)/Formula/ahasend.rb"
brew install ahasend/tap/ahasend

echo "Running brew test"
brew test ahasend/tap/ahasend

# GoReleaser's formula always has a version, which brew audit calls
# redundant; that is the one problem it may report
echo "Running brew audit --strict"
if ! brew audit --strict ahasend/tap/ahasend > "$tmp/audit" 2>&1; then
	cat "$tmp/audit"
	grep -q 'is redundant with version scanned from URL' "$tmp/audit" ||
		fail "brew audit failed"
	if grep '^ *\* ' "$tmp/audit" | grep -v 'is redundant with version scanned from URL' | grep -q .; then
		fail "brew audit reports problems"
	fi
else
	cat "$tmp/audit"
fi

echo "Checking the installed binary"
prefix=$(brew --prefix)
"$prefix/bin/ahasend" --version > "$tmp/version"
cat "$tmp/version"
sed -n 1p "$tmp/version" | grep -qx 'AhaSend CLI v99.0.0' || fail "ahasend prints the wrong version"

echo "Checking the installed files"
keg=$(brew --prefix ahasend/tap/ahasend)
for file in "$prefix/etc/bash_completion.d/ahasend" \
	"$prefix/share/zsh/site-functions/_ahasend" \
	"$prefix/share/fish/vendor_completions.d/ahasend.fish" \
	"$keg/share/doc/ahasend/LICENSE" \
	"$keg/share/doc/ahasend/THIRD_PARTY_NOTICES"; do
	[ -s "$file" ] || fail "$file is missing or empty"
done

echo "Uninstalling the formula"
brew uninstall ahasend/tap/ahasend
[ ! -e "$prefix/bin/ahasend" ] || fail "brew uninstall left $prefix/bin/ahasend"

echo "The Homebrew formula works"
