#!/bin/sh
# Tests scripts/check-rebuild.sh on two copies of a checksums.txt.
set -eu
. test/release/lib.sh

cat > "$tmp/release.txt" <<'EOF'
1111111111111111111111111111111111111111111111111111111111111111  ahasend-linux-amd64
2222222222222222222222222222222222222222222222222222222222222222  ahasend_1.2.3_linux_amd64.tar.gz
EOF
cp "$tmp/release.txt" "$tmp/same.txt"
sed 's/^2222/3333/' "$tmp/release.txt" > "$tmp/other.txt"
head -n 1 "$tmp/release.txt" > "$tmp/fewer.txt"
: > "$tmp/empty.txt"

expect_success "the same checksums pass" scripts/check-rebuild.sh "$tmp/same.txt" "$tmp/release.txt"
expect_error "a different checksum fails" "the rebuilt assets differ from the release's" \
	scripts/check-rebuild.sh "$tmp/other.txt" "$tmp/release.txt"
expect_error "a missing asset fails" "the rebuilt assets differ from the release's" \
	scripts/check-rebuild.sh "$tmp/fewer.txt" "$tmp/release.txt"
expect_error "an empty file fails" "is missing or empty" \
	scripts/check-rebuild.sh "$tmp/empty.txt" "$tmp/release.txt"
expect_error "a missing file fails" "is missing or empty" \
	scripts/check-rebuild.sh "$tmp/same.txt" "$tmp/none.txt"

finish
