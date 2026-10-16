#!/bin/sh
# Tests scripts/winget-manifest.sh on the checksums.txt of a test release
# v1.2.3.
set -eu
. test/release/lib.sh

x64=1111111111111111111111111111111111111111111111111111111111111111
arm64=2222222222222222222222222222222222222222222222222222222222222222
mkdir "$tmp/assets" "$tmp/no-arm64"
printf '%s  %s\n' "$x64" ahasend_1.2.3_windows_amd64.zip "$arm64" ahasend_1.2.3_windows_arm64.zip \
	> "$tmp/assets/checksums.txt"
head -n 1 "$tmp/assets/checksums.txt" > "$tmp/no-arm64/checksums.txt"

export AHASEND_RELEASES_URL='https://releases.example.com/a&b'
manifest() {
	"$root/scripts/winget-manifest.sh" "$@"
}
installer=$tmp/out/AhaSend.CLI.installer.yaml

expect_success "a release's checksums.txt makes the manifests" manifest v1.2.3 "$tmp/assets" "$tmp/out"
expect_output "it writes the three manifests" "AhaSend.CLI.installer.yaml
AhaSend.CLI.locale.en-US.yaml
AhaSend.CLI.yaml" ls "$tmp/out"
expect_output "every manifest has the version" "3" sh -c "grep -l '^PackageVersion: 1.2.3$' '$tmp/out'/*.yaml | wc -l | tr -d ' '"
expect_success "no manifest has a placeholder left" test -z "$(grep @ "$tmp/out"/*.yaml || true)"
expect_success "the x64 installer is the amd64 zip" grep -qxF \
	"    InstallerUrl: https://releases.example.com/a&b/download/v1.2.3/ahasend_1.2.3_windows_amd64.zip" "$installer"
expect_success "the x64 installer has its SHA-256" grep -qxF "    InstallerSha256: $x64" "$installer"
expect_success "the arm64 installer is the arm64 zip" grep -qxF \
	"    InstallerUrl: https://releases.example.com/a&b/download/v1.2.3/ahasend_1.2.3_windows_arm64.zip" "$installer"
expect_success "the arm64 installer has its SHA-256" grep -qxF "    InstallerSha256: $arm64" "$installer"
expect_error "a checksums.txt without a zip fails" "has no line for ahasend_1.2.3_windows_arm64.zip" \
	manifest v1.2.3 "$tmp/no-arm64" "$tmp/out-no-arm64"
expect_error "a tag that is not a release tag fails" "v1.2 is not a release tag" \
	manifest v1.2 "$tmp/assets" "$tmp/out-bad-tag"

finish
