#!/bin/sh
# Commits the files that GoReleaser wrote into dist/ for a channel to the
# channel's git repository (see channel-repo.sh), under their published names:
#
#   homebrew  dist/homebrew/Formula/ahasend.rb as Formula/ahasend.rb
#   scoop     dist/scoop/ahasend.json as ahasend.json
#   aur       dist/aur/ahasend-bin.pkgbuild and .srcinfo as PKGBUILD and
#             .SRCINFO
#
# The repository may be empty, as the AUR repo of a new package is; the
# first push then creates its branch. Run it from the folder that holds
# dist/; it needs jq.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <channel>" >&2
	exit 1
fi
channel=$1

channel_repo_script=$(dirname "$0")/channel-repo.sh
# shellcheck source=scripts/channel-repo.sh
. "$channel_repo_script"
channel_repo "$channel"
version=$(jq -r .version dist/metadata.json)

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
repo=$tmp/repo

git clone -q "$repo_url" "$repo"
if [ -z "$(git -C "$repo" branch -r)" ]; then
	# An empty repository has no branch to start from
	git -C "$repo" checkout -q --orphan "$repo_branch"
else
	git -C "$repo" checkout -q -B "$repo_branch" "origin/$repo_branch"
fi

# put SRC DST copies the generated file SRC to DST in the repository
put() {
	mkdir -p "$(dirname "$repo/$2")"
	cp "$1" "$repo/$2"
	git -C "$repo" add "$2"
}
case $channel in
homebrew) put dist/homebrew/Formula/ahasend.rb Formula/ahasend.rb ;;
scoop) put dist/scoop/ahasend.json ahasend.json ;;
aur)
	put dist/aur/ahasend-bin.pkgbuild PKGBUILD
	put dist/aur/ahasend-bin.srcinfo .SRCINFO
	;;
esac

if git -C "$repo" diff --cached --quiet; then
	echo "$repo_url already has the files of ahasend $version"
	exit 0
fi
git -C "$repo" -c user.name=AhaSend -c user.email=info@ahasend.com commit -q -m "ahasend $version"
git -C "$repo" push -q origin "HEAD:refs/heads/$repo_branch"
echo "Pushed ahasend $version to $repo_url"
