#!/bin/sh
# Tests the release build in a committed copy of the work tree: a stable tag
# on a commit that also has a pre-release tag builds the stable version, and
# make release-snapshot refuses an untracked file when CI is set.
set -eu
. test/release/lib.sh

goreleaser_version=$(sed -n 's/^GORELEASER_VERSION := //p' Makefile)
go_toolchain=$(sed -n 's/^toolchain //p' go.mod)

# The tracked and untracked files of the work tree, committed in a new
# repository with two tags on the commit
src=$tmp/src
mkdir "$src"
git ls-files -co --exclude-standard | tar -cf - -T - | tar -xf - -C "$src"
git init -q "$src"
git -C "$src" add -A
git -C "$src" -c user.name=test -c user.email=test@example.com -c commit.gpgsign=false \
	commit -q -m test
# GoReleaser reads the remote's URL, though it publishes nothing here
git -C "$src" remote add origin https://github.com/AhaSend/ahasend-cli.git
git -C "$src" tag v1.2.3-rc.1
git -C "$src" tag v1.2.3

# build_tag TAG builds the release of TAG without publishing it into
# $tmp/staged, and prints the version of the host's raw binary
build_tag() {
	rm -rf "$tmp/staged"
	(
		cd "$src"
		export GORELEASER_CURRENT_TAG="$1" MAKE_LATEST=false
		eval "$(scripts/build-info.sh | sed 's/^/export /')"
		GOTOOLCHAIN=$go_toolchain go run "github.com/goreleaser/goreleaser/v2@$goreleaser_version" \
			release --clean --skip=publish,homebrew,scoop,aur > "$tmp/goreleaser.log" 2>&1 ||
			{ cat "$tmp/goreleaser.log" >&2; exit 1; }
		scripts/stage-release.sh dist "$tmp/staged" > /dev/null
		"$tmp/staged/ahasend-$(go env GOOS)-$(go env GOARCH)" --version | head -n 1
	)
}
expect_output "the stable tag of a commit with a pre-release tag builds the stable version" \
	"AhaSend CLI v1.2.3" build_tag v1.2.3

# packages TAG builds TAG and prints the names of its .deb and .rpm files
packages() {
	build_tag "$1" > /dev/null || return
	for package in "$tmp/staged"/*.deb "$tmp/staged"/*.rpm; do
		basename "$package"
	done
}
expect_output "a pre-release's packages keep the tag's version in their names" \
	"ahasend_1.2.3-rc.1_amd64.deb
ahasend_1.2.3-rc.1_arm64.deb
ahasend-1.2.3-rc.1-1.aarch64.rpm
ahasend-1.2.3-rc.1-1.x86_64.rpm" packages v1.2.3-rc.1

# snapshot_with_untracked_file CI builds a snapshot with an untracked file,
# with GoReleaser replaced by true, and with CI set to the given value
snapshot_with_untracked_file() {
	touch "$src/notes.txt"
	CI=$1 make -s -C "$src" release-snapshot GORELEASER=true
}
expect_error "an untracked file stops a snapshot in CI" \
	"The work tree has changes or untracked files" snapshot_with_untracked_file true
warns() {
	output=$(snapshot_with_untracked_file "" 2>&1) &&
		echo "$output" | grep -q "^Warning: the work tree has changes or untracked files"
}
expect_success "an untracked file gives a warning outside CI" warns

finish
