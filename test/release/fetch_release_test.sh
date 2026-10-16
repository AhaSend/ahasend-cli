#!/bin/sh
# Tests scripts/fetch-release.sh against the fake release server
# (test/install/fake-release), serving a small test release v1.2.3 and
# copies of it that are broken in one way each. It builds the server with Go
# and needs no network.
set -eu
. test/release/lib.sh

go build -o "$tmp/fake-release" ./test/install/fake-release

release=$tmp/release
mkdir -p "$release/v1.2.3"
printf 'a binary\n' > "$release/v1.2.3/ahasend-linux-amd64"
printf 'a licence\n' > "$release/v1.2.3/LICENSE"
(cd "$release/v1.2.3" && sha256sum ahasend-linux-amd64 LICENSE > checksums.txt)
# v1.2.4: an asset does not match checksums.txt
cp -R "$release/v1.2.3" "$release/v1.2.4"
printf 'changed\n' >> "$release/v1.2.4/LICENSE"
# v1.2.5: checksums.txt lists an asset that is missing
cp -R "$release/v1.2.3" "$release/v1.2.5"
rm "$release/v1.2.5/LICENSE"
# v1.2.6: checksums.txt is empty
mkdir "$release/v1.2.6"
: > "$release/v1.2.6/checksums.txt"
# v1.2.7: checksums.txt lists a path, not an asset name
cp -R "$release/v1.2.3" "$release/v1.2.7"
sed 's/  LICENSE$/  ..\/LICENSE/' "$release/v1.2.3/checksums.txt" > "$release/v1.2.7/checksums.txt"

"$tmp/fake-release" -url "$tmp/url" -latest "$tmp/latest" "$release" 2> "$tmp/server.log" &
server_pid=$!
trap 'kill "$server_pid" 2> /dev/null || true; rm -rf "$tmp"' EXIT
tries=0
while [ ! -s "$tmp/url" ]; do
	tries=$((tries + 1))
	if [ "$tries" -gt 100 ]; then
		echo "the fake release server did not start:" >&2
		cat "$tmp/server.log" >&2
		exit 1
	fi
	sleep 0.1
done
AHASEND_RELEASES_URL=$(cat "$tmp/url")
export AHASEND_RELEASES_URL

fetch() {
	"$root/scripts/fetch-release.sh" "$1" "$tmp/out-$1"
}

expect_output "a release whose assets match passes" "Downloaded the 2 assets of v1.2.3 into $tmp/out-v1.2.3" \
	fetch v1.2.3
expect_success "the downloaded assets are the release's" \
	diff -r "$release/v1.2.3" "$tmp/out-v1.2.3"
expect_error "a changed asset fails" "an asset of v1.2.4 does not match its checksums.txt" fetch v1.2.4
expect_error "a missing asset fails" "404" fetch v1.2.5
expect_error "an empty checksums.txt fails" "the checksums.txt of v1.2.6 is empty" fetch v1.2.6
expect_error "a path in checksums.txt fails" "checksums.txt lists ../LICENSE, which is not an asset name" \
	fetch v1.2.7
expect_error "a tag with no release fails" "404" fetch v1.2.8

finish
