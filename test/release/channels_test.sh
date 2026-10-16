#!/bin/sh
# Tests scripts/channels.sh in a repository with the tags v1.2.3 and v1.3.0.
set -eu
. test/release/lib.sh

scratch_repo "$tmp/repo" v1.2.3 v1.3.0
cd "$tmp/repo"

all="homebrew
scoop
aur
winget
chocolatey
npm"
expect_output "a latest tag publishes every channel" "$all" "$root/scripts/channels.sh" v1.3.0
expect_output "an older tag publishes every channel" "$all" "$root/scripts/channels.sh" v1.2.3
expect_output "a pre-release publishes none" "" "$root/scripts/channels.sh" v1.3.1-rc.1
expect_failure "a tag that is not a release tag fails" "$root/scripts/channels.sh" v1.2

finish
