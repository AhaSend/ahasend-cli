#!/bin/sh
# Prints whether a published channel already has a version: published or
# missing, and for winget also no-package. It does not check npm, because
# npm-publish.mjs checks each npm package itself.
#
# homebrew, scoop, aur: published when the channel has that version or a
# newer one. They keep one version, so a retry of an older tag never moves
# them back. It reads the version in Formula/ahasend.rb of the tap,
# ahasend.json of the bucket, or .SRCINFO of the AUR repo (see
# channel-repo.sh); a repository without that file, such as the empty repo of
# an AUR package that does not exist yet, is missing.
#
# winget: published when `komac list-versions` lists the version, or when an
# open PR to microsoft/winget-pkgs has the package and the version, each as a
# whole word, in its title (`gh search prs`), as Komac's PR titles do. It
# prints no-package when Komac reports that the package does not exist;
# Komac reports any failed request for the package's versions that way too.
# komac reads GITHUB_TOKEN and gh reads GH_TOKEN.
#
# chocolatey: published when the package's page for the version exists on
# community.chocolatey.org, which it also does while the version waits in
# moderation, and missing when the page is not found.
#
# AHASEND_WINGET_ID and AHASEND_CHOCOLATEY_ID replace the package names
# AhaSend.CLI and ahasend, so tests can check a package that exists.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <channel> <version>" >&2
	exit 1
fi
channel=$1
version=$2

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

case $channel in
winget)
	package=${AHASEND_WINGET_ID:-AhaSend.CLI}
	if ! NO_COLOR=1 komac list-versions "$package" > "$tmp/versions" 2> "$tmp/komac.log"; then
		if grep -q "does not exist in microsoft/winget-pkgs" "$tmp/komac.log"; then
			echo "$package is not in microsoft/winget-pkgs" >&2
			echo no-package
			exit 0
		fi
		cat "$tmp/komac.log" >&2
		echo "komac list-versions $package failed" >&2
		exit 1
	fi
	if grep -qxF "$version" "$tmp/versions"; then
		echo "microsoft/winget-pkgs has $package $version" >&2
		echo published
		exit 0
	fi
	gh search prs "$package $version" --repo microsoft/winget-pkgs --state open --match title \
		--json title --jq '.[].title' > "$tmp/titles"
	# A title counts when the package and the version are each a whole word
	# of it, without the punctuation around it, so a PR for 1.2.30 never
	# counts for 1.2.3
	if awk -v package="$package" -v version="$version" '
		{
			has_package = has_version = 0
			n = split($0, words, /[[:space:]]+/)
			for (i = 1; i <= n; i++) {
				word = words[i]
				gsub(/^[^0-9A-Za-z]+|[^0-9A-Za-z]+$/, "", word)
				if (tolower(word) == tolower(package)) has_package = 1
				if (word == version) has_version = 1
			}
			if (has_package && has_version) found = 1
		}
		END { exit !found }' "$tmp/titles"; then
		echo "an open microsoft/winget-pkgs PR adds $package $version" >&2
		echo published
	else
		echo missing
	fi
	exit 0
	;;
chocolatey)
	page=https://community.chocolatey.org/packages/${AHASEND_CHOCOLATEY_ID:-ahasend}/$version
	if ! status=$(curl -sSL --retry 3 -o /dev/null -w '%{http_code}' "$page"); then
		echo "cannot read $page" >&2
		exit 1
	fi
	case $status in
	200)
		echo "$page exists" >&2
		echo published
		;;
	404) echo missing ;;
	*)
		echo "$page returned HTTP $status" >&2
		exit 1
		;;
	esac
	exit 0
	;;
npm)
	echo "npm-publish.mjs checks each npm package itself" >&2
	exit 1
	;;
esac

channel_repo_script=$(dirname "$0")/channel-repo.sh
# shellcheck source=scripts/channel-repo.sh
. "$channel_repo_script"
channel_repo "$channel"

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
