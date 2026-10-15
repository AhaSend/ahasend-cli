#!/bin/sh
# Tests `make check-versions` on copies of the Makefile, go.mod, the
# Dockerfile and the workflows, each with one version changed.
set -eu
. test/release/lib.sh

# copy NAME copies the files into $tmp/NAME
copy() {
	mkdir -p "$tmp/$1/.github"
	cp Makefile go.mod Dockerfile "$tmp/$1/"
	cp -R .github/workflows "$tmp/$1/.github/"
}

check_versions() {
	make -s -C "$tmp/$1" check-versions
}

copy same
expect_success "the repository's versions pass" check_versions same

copy dockerfile
sed -i.bak 's/golang:[0-9.]*-alpine/golang:1.26.0-alpine/' "$tmp/dockerfile/Dockerfile"
expect_error "a Dockerfile with another Go version fails" \
	"Dockerfile does not build with Go" check_versions dockerfile

copy go-version
sed -i.bak "s/GO_VERSION: '[0-9.]*'/GO_VERSION: '1.26.0'/" "$tmp/go-version/.github/workflows/ci.yml"
expect_error "a workflow with another GO_VERSION fails" \
	"ci.yml does not set GO_VERSION" check_versions go-version

copy setup-go
sed -i.bak 's/go-version: .*/go-version: 1.26.0/' "$tmp/setup-go/.github/workflows/ci.yml"
expect_error "a workflow with another go-version fails" \
	"ci.yml uses a Go other than" check_versions setup-go

copy gotoolchain
sed -i.bak "s/GOTOOLCHAIN: '[^']*'/GOTOOLCHAIN: 'go1.26.0'/" "$tmp/gotoolchain/.github/workflows/release.yml"
expect_error "a workflow with another GOTOOLCHAIN fails" \
	"release.yml sets a GOTOOLCHAIN other than" check_versions gotoolchain

copy goreleaser
sed -i.bak "s/GORELEASER_VERSION: '[^']*'/GORELEASER_VERSION: 'v2.17.0'/" "$tmp/goreleaser/.github/workflows/release.yml"
expect_error "a workflow with another GoReleaser version fails" \
	"release.yml does not set GORELEASER_VERSION" check_versions goreleaser

copy action-version
sed -i.bak 's/version: ${{ env.GORELEASER_VERSION }}/version: v2.17.0/' "$tmp/action-version/.github/workflows/release.yml"
expect_error "a goreleaser-action step with another version fails" \
	"release.yml runs goreleaser-action without version" check_versions action-version

copy no-goreleaser-version
cat > "$tmp/no-goreleaser-version/.github/workflows/extra.yml" <<'WORKFLOW'
jobs:
  build:
    steps:
      - uses: goreleaser/goreleaser-action@v6
WORKFLOW
expect_error "a workflow that runs GoReleaser without GORELEASER_VERSION fails" \
	"extra.yml does not set GORELEASER_VERSION" check_versions no-goreleaser-version

finish
