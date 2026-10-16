#!/bin/sh
# Tests scripts/goreleaser-check.sh on .goreleaser.yaml and on the fixtures in
# fixtures/goreleaser-check.
set -eu
. test/release/lib.sh

configs=$fixtures/goreleaser-check

expect_success "the release configuration passes" scripts/goreleaser-check.sh
# CI is set on GitHub Actions, where GoReleaser colours its output; each case
# runs with and without it
for ci in "" true; do
	expect_success "brews as the only deprecated property passes (CI=$ci)" \
		env CI="$ci" scripts/goreleaser-check.sh "$configs/brews.yaml"
	expect_error "another deprecated property fails (CI=$ci)" \
		"deprecated properties other than brews: archives.format" \
		env CI="$ci" scripts/goreleaser-check.sh "$configs/brews-and-archives-format.yaml"
done

finish
