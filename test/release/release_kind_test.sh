#!/bin/sh
# Tests scripts/release-kind.sh, scripts/make-latest.sh and
# scripts/docker-tags.sh in a repository with the tags v1.2.3 and v1.3.0.
set -eu
. test/release/lib.sh

scratch_repo "$tmp/repo" v1.2.3 v1.3.0
cd "$tmp/repo"

expect_output "release-kind: the newest stable tag is latest" latest "$root/scripts/release-kind.sh" v1.3.0
expect_output "release-kind: a lower stable tag is older" older "$root/scripts/release-kind.sh" v1.2.3
expect_output "release-kind: a tag with a suffix is a pre-release" prerelease "$root/scripts/release-kind.sh" v1.3.1-rc.1
for tag in 1.2.3 v1.2 v1.2-rc.1 v1.2.3.4; do
	expect_failure "release-kind: $tag is not a release tag" "$root/scripts/release-kind.sh" "$tag"
done

expect_output "make-latest: latest is marked latest" true "$root/scripts/make-latest.sh" v1.3.0
expect_output "make-latest: older is not" false "$root/scripts/make-latest.sh" v1.2.3
expect_output "make-latest: a pre-release is not" false "$root/scripts/make-latest.sh" v1.3.1-rc.1

expect_output "docker-tags: latest gets latest" "v1.3.0
1.3.0
latest" "$root/scripts/docker-tags.sh" v1.3.0
expect_output "docker-tags: older gets no latest" "v1.2.3
1.2.3" "$root/scripts/docker-tags.sh" v1.2.3
expect_output "docker-tags: a pre-release gets no latest" "v1.3.1-rc.1
1.3.1-rc.1" "$root/scripts/docker-tags.sh" v1.3.1-rc.1

finish
