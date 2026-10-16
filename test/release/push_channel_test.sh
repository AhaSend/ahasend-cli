#!/bin/sh
# Tests scripts/push-channel.sh with the files in fixtures/channels/dist,
# against local repositories that stand in for the tap, the bucket and the
# AUR repo: empty ones, and ones that hold an older version.
set -eu
. test/release/lib.sh

# The developer's git settings must not change the results
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1

dist=$fixtures/channels/dist

# push CHANNEL REPO runs push-channel.sh in the folder of the fixture dist/,
# with the channel's repository replaced by the local REPO
push() {
	(
		cd "$fixtures/channels"
		AHASEND_TAP_REPO=file://$2 AHASEND_BUCKET_REPO=file://$2 AHASEND_AUR_REPO=file://$2 \
			"$root/scripts/push-channel.sh" "$1"
	)
}

# pushed CHANNEL REPO BRANCH SRC DST... pushes CHANNEL to REPO, and checks
# that BRANCH of REPO then has each file SRC at the path DST, in a commit
# named after the version
pushed() {
	channel=$1
	repo=$2
	branch=$3
	shift 3
	push "$channel" "$repo" || return
	while [ $# -gt 0 ]; do
		if ! git --git-dir="$repo" show "$branch:$2" | cmp -s - "$1"; then
			echo "$branch:$2 in $repo is not $1"
			return 1
		fi
		shift 2
	done
	message=$(git --git-dir="$repo" log -1 --format=%s "$branch")
	if [ "$message" != "ahasend 1.2.3" ]; then
		echo "the commit is named \"$message\""
		return 1
	fi
}

bare_repo "$tmp/empty-tap.git" main
expect_success "homebrew: the formula goes to Formula/ in an empty tap" \
	pushed homebrew "$tmp/empty-tap.git" main "$dist/homebrew/Formula/ahasend.rb" Formula/ahasend.rb
bare_repo "$tmp/empty-bucket.git" main
expect_success "scoop: the manifest goes to an empty bucket" \
	pushed scoop "$tmp/empty-bucket.git" main "$dist/scoop/ahasend.json" ahasend.json
bare_repo "$tmp/empty-aur.git" master
expect_success "aur: PKGBUILD and .SRCINFO go to the master branch of an empty AUR repo" \
	pushed aur "$tmp/empty-aur.git" master "$dist/aur/ahasend-bin.pkgbuild" PKGBUILD \
	"$dist/aur/ahasend-bin.srcinfo" .SRCINFO

# Repositories with the files of 1.2.2 and a README
echo "The tap" > "$tmp/README.md"
for file in homebrew/Formula/ahasend.rb scoop/ahasend.json aur/ahasend-bin.pkgbuild aur/ahasend-bin.srcinfo; do
	mkdir -p "$tmp/old/$(dirname "$file")"
	sed 's/1\.2\.3/1.2.2/g' "$dist/$file" > "$tmp/old/$file"
done
bare_repo "$tmp/tap.git" main "$tmp/old/homebrew/Formula/ahasend.rb" Formula/ahasend.rb "$tmp/README.md" README.md
bare_repo "$tmp/bucket.git" main "$tmp/old/scoop/ahasend.json" ahasend.json
bare_repo "$tmp/aur.git" master "$tmp/old/aur/ahasend-bin.pkgbuild" PKGBUILD \
	"$tmp/old/aur/ahasend-bin.srcinfo" .SRCINFO
expect_success "homebrew: the formula replaces an older one, and the README stays" \
	pushed homebrew "$tmp/tap.git" main "$dist/homebrew/Formula/ahasend.rb" Formula/ahasend.rb \
	"$tmp/README.md" README.md
expect_success "scoop: the manifest replaces an older one" \
	pushed scoop "$tmp/bucket.git" main "$dist/scoop/ahasend.json" ahasend.json
expect_success "aur: PKGBUILD and .SRCINFO replace older ones" \
	pushed aur "$tmp/aur.git" master "$dist/aur/ahasend-bin.pkgbuild" PKGBUILD \
	"$dist/aur/ahasend-bin.srcinfo" .SRCINFO

expect_output "the pushed version is then published" published \
	env AHASEND_TAP_REPO="file://$tmp/tap.git" "$root/scripts/channel-published.sh" homebrew 1.2.3

# same_files pushes the same files again, which makes no new commit
same_files() {
	before=$(git --git-dir="$tmp/tap.git" rev-parse main)
	push homebrew "$tmp/tap.git" | grep -q "already has the files of ahasend 1.2.3" || return
	[ "$(git --git-dir="$tmp/tap.git" rev-parse main)" = "$before" ]
}
expect_success "pushing the same files again makes no commit" same_files

expect_error "another channel fails" "npm is not a channel with a git repository" push npm "$tmp/tap.git"

finish
