#!/bin/sh
# Checks a folder of staged release assets (see stage-release.sh) of a build
# of the given version, 99.0.0 by default, the version of test snapshots:
# the asset names, checksums.txt, the version the linux amd64 binary prints,
# and the files in the archives and the .deb and .rpm packages. It needs
# sha256sum, tar, unzip, dpkg-deb and rpm, and runs on linux amd64.
set -eu

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
	echo "usage: $0 <staged> [<version>]" >&2
	exit 1
fi
staged=$1
version=${2:-99.0.0}

fail() {
	echo "check-release-assets: $*" >&2
	exit 1
}

# expect_lines NAME EXPECTED ACTUAL fails unless both lists hold the same
# lines, in any order
expect_lines() {
	if [ "$(printf '%s\n' "$2" | sort)" != "$(printf '%s\n' "$3" | sort)" ]; then
		printf '%s\n' "$2" | sort > "$tmp/expected"
		printf '%s\n' "$3" | sort > "$tmp/actual"
		diff -u "$tmp/expected" "$tmp/actual" >&2 || true
		fail "$1 differ from the expected ones"
	fi
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

echo "Checking the asset names"
expected="checksums.txt
LICENSE
THIRD_PARTY_NOTICES"
for platform in linux-amd64 linux-arm64 darwin-amd64 darwin-arm64; do
	expected="$expected
ahasend-$platform
ahasend_${version}_$(echo "$platform" | tr - _).tar.gz"
done
for arch in amd64 arm64; do
	expected="$expected
ahasend-windows-$arch.exe
ahasend_${version}_windows_$arch.zip
ahasend_${version}_$arch.deb"
done
expected="$expected
ahasend-$version-1.x86_64.rpm
ahasend-$version-1.aarch64.rpm"
names=$(ls "$staged")
expect_lines "the asset names" "$expected" "$names"

echo "Checking checksums.txt"
expect_lines "the names in checksums.txt" "$(echo "$names" | grep -vx checksums.txt)" \
	"$(awk '{ print $2 }' "$staged/checksums.txt")"
(cd "$staged" && sha256sum -c --quiet checksums.txt) || fail "checksums.txt does not match the assets"

echo "Checking the version the binary prints"
"$staged/ahasend-linux-amd64" --version > "$tmp/version"
cat "$tmp/version"
sed -n 1p "$tmp/version" | grep -qx "AhaSend CLI v$version" || fail "the binary prints the wrong version"
sed -n 2p "$tmp/version" | grep -Eqx 'Build Time: [0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z' ||
	fail "the binary prints no RFC 3339 build time"
sed -n 3p "$tmp/version" | grep -Eqx 'Git Commit: [0-9a-f]{7}' || fail "the binary prints no 7-character commit"

echo "Checking the archives"
for platform in linux_amd64 linux_arm64 darwin_amd64 darwin_arm64; do
	expect_lines "the files in ahasend_${version}_$platform.tar.gz" "-rwxr-xr-x root/root ahasend
-rw-r--r-- root/root LICENSE
-rw-r--r-- root/root THIRD_PARTY_NOTICES
-rw-r--r-- root/root completions/ahasend.bash
-rw-r--r-- root/root completions/_ahasend
-rw-r--r-- root/root completions/ahasend.fish" \
		"$(tar -tvzf "$staged/ahasend_${version}_$platform.tar.gz" | awk '{ print $1, $2, $6 }')"
done
for arch in amd64 arm64; do
	expect_lines "the files in ahasend_${version}_windows_$arch.zip" "-rwxr-xr-x ahasend.exe
-rw-r--r-- LICENSE
-rw-r--r-- THIRD_PARTY_NOTICES" \
		"$(unzip -Z "$staged/ahasend_${version}_windows_$arch.zip" | awk '/^[-dl]/ { print $1, $NF }')"
done

echo "Checking the .deb and .rpm packages"
deb_files="/usr/bin/ahasend
/usr/share/bash-completion/completions/ahasend
/usr/share/fish/vendor_completions.d/ahasend.fish
/usr/share/zsh/vendor-completions/_ahasend
/usr/share/doc/ahasend/LICENSE
/usr/share/doc/ahasend/THIRD_PARTY_NOTICES"
rpm_files=$(echo "$deb_files" | sed 's|/zsh/vendor-completions/|/zsh/site-functions/|')
for arch in amd64 arm64; do
	expect_lines "the files in ahasend_${version}_$arch.deb" "$deb_files" \
		"$(dpkg-deb -c "$staged/ahasend_${version}_$arch.deb" | awk '!/^d/ { sub(/^\./, "", $6); print $6 }')"
done
for arch in x86_64 aarch64; do
	expect_lines "the files in ahasend-$version-1.$arch.rpm" "$rpm_files" \
		"$(rpm -qlp "$staged/ahasend-$version-1.$arch.rpm")"
done

echo "The release assets are correct"
