#!/bin/sh
# Tests scripts/release-notes.sh on fixtures/release-notes/CHANGELOG.md.
set -eu
. test/release/lib.sh

scratch_repo "$tmp/repo" v1.2.3
cd "$tmp/repo"
changelog=$fixtures/release-notes/CHANGELOG.md
blob=https://github.com/AhaSend/ahasend-cli/blob

notes_1_2_3="### Changed

- Exit codes follow the [Exit Codes]($blob/v1.2.3/USAGE.mdx#exit-codes) table.
- Output follows [the guide][guide], as on [the site](https://ahasend.com/docs).

[guide]: $blob/v1.2.3/docs/guide.md"
expect_output "a stable tag's dated section, without its heading" "$notes_1_2_3" \
	"$root/scripts/release-notes.sh" v1.2.3 "$changelog"

sed 's/^## \[1\.2\.3\] - .*/## [1.2.3]/' "$changelog" > "$tmp/undated.md"
expect_output "a stable tag's section without a date" "$notes_1_2_3" \
	"$root/scripts/release-notes.sh" v1.2.3 "$tmp/undated.md"

notes_1_3_0="A paragraph that is wrapped over two lines.

### Changed

- An item that is wrapped, with a [link]($blob/v1.3.0/USAGE.mdx#a) on its last line.
  - A nested item that is wrapped.

  A second paragraph of the item.

\`\`\`sh
keep
  these lines
\`\`\`

| a | b |
|---|---|"
expect_output "wrapped paragraphs and items become one line each, code and tables stay" \
	"$notes_1_3_0" "$root/scripts/release-notes.sh" v1.3.0 "$changelog"

expect_output "a pre-release's Unreleased section" "### Added

- Pre-release notes." "$root/scripts/release-notes.sh" v1.2.4-rc.1 "$changelog"

expect_error "a version with no section fails" "has no ## [1.2.4] section" \
	"$root/scripts/release-notes.sh" v1.2.4 "$changelog"
expect_error "an empty section fails" "has no ## [1.2.2] section, or it is empty" \
	"$root/scripts/release-notes.sh" v1.2.2 "$changelog"
expect_error "a tag that is not a release tag fails" "is not a release tag" \
	"$root/scripts/release-notes.sh" v1.2 "$changelog"

finish
