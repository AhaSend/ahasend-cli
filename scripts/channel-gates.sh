#!/bin/sh
# Checks that publish-channel.yml may publish a channel for a release tag. It
# fails, naming the check, when the tag is a pre-release, when the run's ref
# is not refs/tags/<tag> (the templates and scripts would then not be the
# tag's), or when `gh release view` finds no published release for the tag.
# A token that cannot see drafts finds no release for a draft, and a draft
# it can see fails too.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <tag> <ref>" >&2
	exit 1
fi
tag=$1
ref=$2

kind=$("$(dirname "$0")/release-kind.sh" "$tag")
if [ "$kind" = prerelease ]; then
	echo "$tag is a pre-release; channels publish stable releases only" >&2
	exit 1
fi
if [ "$ref" != "refs/tags/$tag" ]; then
	echo "the run's ref is $ref, not refs/tags/$tag; start it with --ref $tag" >&2
	exit 1
fi
if ! draft=$(gh release view "$tag" --json isDraft --jq .isDraft); then
	echo "$tag has no published GitHub release" >&2
	exit 1
fi
if [ "$draft" != false ]; then
	echo "the GitHub release of $tag is a draft, not published" >&2
	exit 1
fi
