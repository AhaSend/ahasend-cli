#!/bin/sh
# Tests scripts/choco-package.sh on the assets of a small test release
# v1.2.3, and on copies of them that are broken in one way each.
set -eu
. test/release/lib.sh

assets=$tmp/assets
mkdir "$assets"
printf 'an exe\n' > "$assets/ahasend-windows-amd64.exe"
printf 'a licence\n' > "$assets/LICENSE"
printf 'notices\n' > "$assets/THIRD_PARTY_NOTICES"
(cd "$assets" && sha256sum ahasend-windows-amd64.exe LICENSE > checksums.txt)
sha256=$(awk '$2 == "ahasend-windows-amd64.exe" { print $1 }' "$assets/checksums.txt")
# changed/: the exe does not match checksums.txt
cp -R "$assets" "$tmp/changed"
printf 'changed\n' >> "$tmp/changed/ahasend-windows-amd64.exe"
# back\slash/: the assets in a folder whose path has a backslash, as a
# Windows path does
cp -R "$assets" "$tmp/back\\slash"
# no-line/: checksums.txt has no line for the exe
cp -R "$assets" "$tmp/no-line"
grep -v ahasend-windows-amd64.exe "$assets/checksums.txt" > "$tmp/no-line/checksums.txt"

export AHASEND_RELEASES_URL=https://releases.example.com/ahasend
package() {
	"$root/scripts/choco-package.sh" "$@"
}

expect_success "a release's assets make a package" package v1.2.3 "$assets" "$tmp/out"
expect_success "the nuspec has the version" grep -qF '<version>1.2.3</version>' "$tmp/out/ahasend.nuspec"
expect_success "the nuspec has no placeholder left" test -z "$(grep @ "$tmp/out/ahasend.nuspec" || true)"
expect_success "VERIFICATION.txt gives the exe's release URL" grep -qF \
	"  https://releases.example.com/ahasend/download/v1.2.3/ahasend-windows-amd64.exe" \
	"$tmp/out/tools/VERIFICATION.txt"
expect_success "VERIFICATION.txt gives the exe's SHA-256 from checksums.txt" grep -qxF "  $sha256" \
	"$tmp/out/tools/VERIFICATION.txt"
expect_success "VERIFICATION.txt has no placeholder left" \
	test -z "$(grep @ "$tmp/out/tools/VERIFICATION.txt" || true)"
expect_success "the exe is the asset" cmp "$assets/ahasend-windows-amd64.exe" "$tmp/out/tools/ahasend.exe"
expect_success "LICENSE.txt is the asset" cmp "$assets/LICENSE" "$tmp/out/tools/LICENSE.txt"
expect_success "THIRD_PARTY_NOTICES.txt is the asset" \
	cmp "$assets/THIRD_PARTY_NOTICES" "$tmp/out/tools/THIRD_PARTY_NOTICES.txt"
expect_success "assets in a folder whose path has a backslash make a package" \
	package v1.2.3 "$tmp/back\\slash" "$tmp/out-backslash"
expect_error "an exe that does not match checksums.txt fails" "ahasend-windows-amd64.exe does not match checksums.txt" \
	package v1.2.3 "$tmp/changed" "$tmp/out-changed"
expect_error "a checksums.txt without the exe fails" "has no line for ahasend-windows-amd64.exe" \
	package v1.2.3 "$tmp/no-line" "$tmp/out-no-line"
expect_error "a tag that is not a release tag fails" "1.2.3 is not a release tag" \
	package 1.2.3 "$assets" "$tmp/out-bad-tag"

finish
