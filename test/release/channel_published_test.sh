#!/bin/sh
# Tests scripts/channel-published.sh against local repositories that stand
# in for the tap, the bucket and the AUR repo, holding the version 1.2.3 of
# the files in fixtures/channels/dist.
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
expect_error "another channel fails" "npm is not a channel with a git repository" \
	published npm "$tmp/tap.git" 1.2.3

finish
