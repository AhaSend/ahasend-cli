#!/bin/sh
# Tests scripts/channel-published.sh against local repositories that stand
# in for the tap, the bucket and the AUR repo, holding the version 1.2.3 of
# the files in fixtures/channels/dist, and with stubs of komac, gh and curl
# that stand in for winget-pkgs and community.chocolatey.org.
# channel_published_network_test.sh checks WinGet and Chocolatey themselves.
set -eu
. test/release/lib.sh

# The developer's git settings must not change the results
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

dist=$fixtures/channels/dist
bare_repo "$tmp/tap.git" main "$dist/homebrew/Formula/ahasend.rb" Formula/ahasend.rb
bare_repo "$tmp/bucket.git" main "$dist/scoop/ahasend.json" ahasend.json
bare_repo "$tmp/aur.git" master "$dist/aur/ahasend-bin.pkgbuild" PKGBUILD \
	"$dist/aur/ahasend-bin.srcinfo" .SRCINFO
bare_repo "$tmp/empty.git" main
echo "A repository without the channel's file" > "$tmp/README.md"
bare_repo "$tmp/no-file-main.git" main "$tmp/README.md" README.md
bare_repo "$tmp/no-file-master.git" master "$tmp/README.md" README.md
sed '/^  version /d' "$dist/homebrew/Formula/ahasend.rb" > "$tmp/no-version.rb"
bare_repo "$tmp/no-version.git" main "$tmp/no-version.rb" Formula/ahasend.rb

# published CHANNEL REPO VERSION runs channel-published.sh for VERSION with
# the channel's repository replaced by the local REPO
published() {
	AHASEND_TAP_REPO=file://$2 AHASEND_BUCKET_REPO=file://$2 AHASEND_AUR_REPO=file://$2 \
		"$root/scripts/channel-published.sh" "$1" "$3"
}

for channel in homebrew scoop aur; do
	case $channel in
	homebrew) repo=$tmp/tap.git no_file=$tmp/no-file-main.git ;;
	scoop) repo=$tmp/bucket.git no_file=$tmp/no-file-main.git ;;
	aur) repo=$tmp/aur.git no_file=$tmp/no-file-master.git ;;
	esac
	expect_output "$channel: the same version is published" published published "$channel" "$repo" 1.2.3
	expect_output "$channel: a newer version is published" published published "$channel" "$repo" 1.2.2
	expect_output "$channel: an older version is missing" missing published "$channel" "$repo" 1.2.4
	expect_output "$channel: versions compare as numbers" missing published "$channel" "$repo" 1.10.0
	expect_output "$channel: an empty repository is missing" missing published "$channel" "$tmp/empty.git" 1.2.3
	expect_output "$channel: a repository without the file is missing" missing \
		published "$channel" "$no_file" 1.2.3
done

expect_error "a file without a version fails" "Formula/ahasend.rb of file://$tmp/no-version.git has no version" \
	published homebrew "$tmp/no-version.git" 1.2.3
expect_error "a repository that does not exist fails" "does-not-exist.git" \
	published homebrew "$tmp/does-not-exist.git" 1.2.3
expect_error "npm fails" "npm-publish.mjs checks each npm package itself" \
	published npm "$tmp/tap.git" 1.2.3
expect_error "another channel fails" "snap is not a channel with a git repository" \
	published snap "$tmp/tap.git" 1.2.3

# The komac stub knows the versions 1.7.1 and 1.8.0 of jqlang.jq and 0.5.0 of
# AhaSend.CLI, and says any other package does not exist, as Komac does; it
# fails for the package bad.token as Komac does for a token it cannot use
mkdir "$tmp/bin"
cat > "$tmp/bin/komac" <<'STUB'
#!/bin/sh
case "$*" in
"list-versions jqlang.jq") printf '1.7.1\n1.8.0\n' ;;
"list-versions AhaSend.CLI") echo 0.5.0 ;;
"list-versions bad.token")
	printf 'Error:\n   0: The provided GitHub token is invalid\n' >&2
	exit 1
	;;
"list-versions "*)
	printf 'Error:\n   0: %s does not exist in microsoft/winget-pkgs\n' "$2" >&2
	exit 1
	;;
*)
	echo "unexpected komac call: $*" >&2
	exit 1
	;;
esac
STUB
# The gh stub answers the search for open winget-pkgs PRs with the titles of
# PRs that add jqlang.jq 1.9.0 and 1.2.30 and another package's 1.5.0; the
# script picks the matching titles itself
cat > "$tmp/bin/gh" <<'STUB'
#!/bin/sh
case "$*" in
"search prs jqlang.jq "*" --repo microsoft/winget-pkgs --state open --match title --json title --jq .[].title")
	echo "New version: jqlang.jq version 1.9.0"
	echo "New version: jqlang.jq version 1.2.30"
	echo "New version: Other.Package version 1.5.0"
	;;
*)
	echo "unexpected gh call: $*" >&2
	exit 1
	;;
esac
STUB
# The curl stub prints the HTTP status of the Chocolatey pages: jq 1.7.1 and
# ahasend 0.5.0 exist, jq 5.0.0 gives a server error, jq 0.0.1 cannot be
# reached, and every other page is not found
cat > "$tmp/bin/curl" <<'STUB'
#!/bin/sh
for url; do :; done
case $url in
https://community.chocolatey.org/packages/jq/1.7.1 | https://community.chocolatey.org/packages/ahasend/0.5.0)
	printf 200
	;;
https://community.chocolatey.org/packages/jq/5.0.0) printf 503 ;;
https://community.chocolatey.org/packages/jq/0.0.1)
	echo "curl: (6) Could not resolve host: community.chocolatey.org" >&2
	exit 6
	;;
*) printf 404 ;;
esac
STUB
chmod 755 "$tmp/bin/komac" "$tmp/bin/gh" "$tmp/bin/curl"

# with_stubs COMMAND... runs COMMAND with the stubs first on PATH
with_stubs() {
	PATH=$tmp/bin:$PATH "$@"
}
check() {
	with_stubs "$root/scripts/channel-published.sh" "$@"
}

export AHASEND_WINGET_ID=jqlang.jq
expect_output "winget: a version Komac lists is published" published check winget 1.8.0
expect_output "winget: a version only an open PR adds is published" published check winget 1.9.0
expect_output "winget: a version with no PR is missing" missing check winget 2.0.0
expect_output "winget: a PR for 1.2.30 does not publish 1.2.3" missing check winget 1.2.3
expect_output "winget: a PR for 1.2.30 publishes 1.2.30" published check winget 1.2.30
expect_output "winget: a version in another package's PR is missing" missing check winget 1.5.0
export AHASEND_WINGET_ID=Nobody.Nothing
expect_output "winget: a package that does not exist is no-package" no-package check winget 1.2.3
export AHASEND_WINGET_ID=bad.token
expect_error "winget: another Komac error fails" "The provided GitHub token is invalid" check winget 1.2.3
unset AHASEND_WINGET_ID
expect_output "winget: the package is AhaSend.CLI" published check winget 0.5.0

export AHASEND_CHOCOLATEY_ID=jq
expect_output "chocolatey: an existing version is published" published check chocolatey 1.7.1
expect_output "chocolatey: a version that is not found is missing" missing check chocolatey 1.7.99
expect_error "chocolatey: a server error fails" "returned HTTP 503" check chocolatey 5.0.0
expect_error "chocolatey: a failed request fails" "cannot read https://community.chocolatey.org/packages/jq/0.0.1" \
	check chocolatey 0.0.1
unset AHASEND_CHOCOLATEY_ID
expect_output "chocolatey: the package is ahasend" published check chocolatey 0.5.0

finish
