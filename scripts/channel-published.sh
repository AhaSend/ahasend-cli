#!/bin/sh
# Prints whether a published channel already has a version: published when
# it has that version or a newer one, and missing otherwise. Homebrew, Scoop
# and AUR keep one version, so a newer one counts as published and a retry of
# an older tag never moves them back. It reads the version in
# Formula/ahasend.rb of the tap, ahasend.json of the bucket, or .SRCINFO of
# the AUR repo (see channel-repo.sh); a repository without that file, such as
# the empty repo of an AUR package that does not exist yet, is missing.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <channel> <version>" >&2
	exit 1
fi
channel=$1
version=$2

channel_repo_script=$(dirname "$0")/channel-repo.sh
# shellcheck source=scripts/channel-repo.sh
. "$channel_repo_script"
channel_repo "$channel"

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# The version the channel has, or nothing when it has none
current=
heads=$(git ls-remote --heads "$repo_url" "refs/heads/$repo_branch")
if [ -n "$heads" ]; then
	git clone -q --depth 1 --branch "$repo_branch" "$repo_url" "$tmp/repo"
	file=$tmp/repo/$repo_file
	if [ -f "$file" ]; then
		case $channel in
		homebrew) current=$(sed -n 's/^  version "\(.*\)"$/\1/p' "$file" | head -n 1) ;;
		scoop) current=$(jq -r '.version // empty' "$file") ;;
		aur) current=$(sed -n 's/^[[:space:]]*pkgver = //p' "$file" | head -n 1) ;;
		esac
		if [ -z "$current" ]; then
			echo "$repo_file of $repo_url has no version" >&2
			exit 1
		fi
	fi
fi

if [ -z "$current" ]; then
	echo "$channel has no version yet" >&2
	echo missing
	exit 0
fi
echo "$channel has $current" >&2
newest=$(printf '%s\n%s\n' "$current" "$version" | sort -V | tail -n 1)
if [ "$newest" = "$current" ]; then
	echo published
else
	echo missing
fi
