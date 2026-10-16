#!/bin/sh
# Tests the WinGet and Chocolatey checks of scripts/channel-published.sh
# against microsoft/winget-pkgs and community.chocolatey.org, with packages
# that exist there: jqlang.jq on WinGet and jq on Chocolatey. It needs the
# network, komac on PATH and a GITHUB_TOKEN that can read public
# repositories, so it runs only with RELEASE_TESTS_NETWORK set, as
# packaging.yml runs it. A gh stub stands in for the search of open PRs.
set -eu
. test/release/lib.sh
skip_unless RELEASE_TESTS_NETWORK "needs the network, komac and GITHUB_TOKEN"

# The gh stub prints the titles in $tmp/prs as the open winget-pkgs PRs
mkdir "$tmp/bin"
cat > "$tmp/bin/gh" <<'STUB'
#!/bin/sh
case "$*" in
"search prs "*" --repo microsoft/winget-pkgs --state open --match title --json title --jq .[].title")
	cat "$PRS"
	;;
*)
	echo "unexpected gh call: $*" >&2
	exit 1
	;;
esac
STUB
chmod 755 "$tmp/bin/gh"
export PRS="$tmp/prs"
: > "$PRS"

check() {
	PATH=$tmp/bin:$PATH "$root/scripts/channel-published.sh" "$@"
}

export AHASEND_WINGET_ID=jqlang.jq
expect_output "winget: a version in winget-pkgs is published" published check winget 1.7.1
expect_output "winget: a version that does not exist is missing" missing check winget 1.2.3
echo "New version: jqlang.jq version 1.2.30" > "$PRS"
expect_output "winget: a PR for 1.2.30 does not publish 1.2.3" missing check winget 1.2.3
echo "New version: jqlang.jq version 1.2.3" >> "$PRS"
expect_output "winget: a version only an open PR adds is published" published check winget 1.2.3
: > "$PRS"
export AHASEND_WINGET_ID=AhaSendTest.DoesNotExist
expect_output "winget: a package that does not exist is no-package" no-package check winget 1.2.3

export AHASEND_CHOCOLATEY_ID=jq
expect_output "chocolatey: an approved version is published" published check chocolatey 1.7.1
expect_output "chocolatey: a version that does not exist is missing" missing check chocolatey 1.7.99

finish
