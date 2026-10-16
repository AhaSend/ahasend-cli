#!/bin/sh
# Tests scripts/npm-publish.mjs against a local Verdaccio registry with no
# uplinks (test/install/verdaccio.yaml), with the packages that
# scripts/npm-packages.mjs makes from small test assets. VERDACCIO names
# Verdaccio's bin/verdaccio script, which runs with node; packaging.yml
# installs Verdaccio and sets it. It needs node, npm, curl and jq.
set -eu
. test/release/lib.sh
skip_unless VERDACCIO "needs Verdaccio"

# npm reads and writes only this test's files, and talks to Verdaccio only
unset NPM_CONFIG_USERCONFIG NPM_TOKEN NODE_AUTH_TOKEN
export HOME="$tmp/home"
export npm_config_userconfig="$HOME/.npmrc" npm_config_cache="$tmp/npm-cache"
export npm_config_registry=http://127.0.0.1:4873/
mkdir "$HOME"

cp test/install/verdaccio.yaml "$tmp/"
node "$VERDACCIO" --config "$tmp/verdaccio.yaml" --listen 127.0.0.1:4873 > "$tmp/verdaccio.log" 2>&1 &
verdaccio_pid=$!
trap 'kill "$verdaccio_pid" 2> /dev/null || true; rm -rf "$tmp"' EXIT
tries=0
until curl -fs "${npm_config_registry}-/ping" > /dev/null 2>&1; do
	tries=$((tries + 1))
	if [ "$tries" -gt 100 ]; then
		echo "Verdaccio did not start:" >&2
		cat "$tmp/verdaccio.log" >&2
		exit 1
	fi
	sleep 0.2
done
# A user of this registry only, who may publish
token=$(curl -fsS -X PUT -H 'Content-Type: application/json' -d '{"name": "test", "password": "test-password"}' \
	"${npm_config_registry}-/user/org.couchdb.user:test" | jq -r .token)
echo "//127.0.0.1:4873/:_authToken=$token" > "$npm_config_userconfig"

# The seven packages of each test version, with small stand-ins for the
# release assets
mkdir "$tmp/assets"
for asset in ahasend-darwin-arm64 ahasend-darwin-amd64 ahasend-linux-arm64 ahasend-linux-amd64 \
	ahasend-windows-arm64.exe ahasend-windows-amd64.exe LICENSE THIRD_PARTY_NOTICES; do
	echo "$asset" > "$tmp/assets/$asset"
done
for version in 1.2.3 1.3.0 1.2.4; do
	scripts/npm-packages.mjs "v$version" "$tmp/assets" "$tmp/packages-$version" > /dev/null
done

# publish VERSION runs npm-publish.mjs on the packages of VERSION and prints
# the lines that say what it publishes
publish() {
	"$root/scripts/npm-publish.mjs" "$tmp/packages-$1" > "$tmp/publish.log" 2>&1 || {
		cat "$tmp/publish.log"
		return 1
	}
	grep -E '^(Publishing|.* older than)' "$tmp/publish.log" || true
}
view() {
	npm view "$@" --prefer-online 2> /dev/null
}

# Three platform packages already have 1.2.3; the other four do not exist
for platform in darwin-arm64 linux-x64 win32-x64; do
	npm publish "$tmp/packages-1.2.3/cli-$platform" --access public > /dev/null 2>&1
done
expect_output "a package that does not exist is missing, and only the missing packages are published, the wrapper last" \
	"Publishing @ahasend/cli-darwin-x64@1.2.3
Publishing @ahasend/cli-linux-arm64@1.2.3
Publishing @ahasend/cli-win32-arm64@1.2.3
Publishing @ahasend/cli@1.2.3" publish 1.2.3
expect_output "every package has 1.2.3" 7 sh -c "for dir in '$tmp'/packages-1.2.3/*/; do
	name=\$(jq -r .name \"\$dir/package.json\")
	npm view \"\$name@1.2.3\" version --prefer-online
done | grep -c '^1.2.3\$'"
expect_output "a repeated run publishes nothing" "" publish 1.2.3

expect_success "a newer version is published" publish 1.3.0
expect_output "the newer version is the latest dist-tag" 1.3.0 view @ahasend/cli dist-tags.latest
expect_output "a version older than the latest dist-tag is published with --tag legacy" \
	"@ahasend/cli-darwin-arm64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-darwin-arm64@1.2.4
@ahasend/cli-darwin-x64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-darwin-x64@1.2.4
@ahasend/cli-linux-arm64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-linux-arm64@1.2.4
@ahasend/cli-linux-x64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-linux-x64@1.2.4
@ahasend/cli-win32-arm64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-win32-arm64@1.2.4
@ahasend/cli-win32-x64@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli-win32-x64@1.2.4
@ahasend/cli@1.2.4 is older than the latest dist-tag, 1.3.0
Publishing @ahasend/cli@1.2.4" publish 1.2.4
expect_output "the latest dist-tag stays on the newer version" 1.3.0 view @ahasend/cli dist-tags.latest
expect_output "the older version has the legacy dist-tag" 1.2.4 view @ahasend/cli-linux-x64 dist-tags.legacy

expect_error "a registry that cannot be reached fails" "npm view @ahasend/cli-darwin-arm64@1.2.3 version failed" \
	env npm_config_registry=http://127.0.0.1:9/ npm_config_fetch_retries=0 \
	"$root/scripts/npm-publish.mjs" "$tmp/packages-1.2.3"
expect_error "a folder without the seven packages fails" "must hold the seven packages" \
	"$root/scripts/npm-publish.mjs" "$tmp/assets"

finish
