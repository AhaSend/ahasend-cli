#!/bin/sh
# Tests scripts/install-komac.sh with a curl stub that serves a test archive
# for the Komac release URL of v2.0.0.
set -eu
. test/release/lib.sh

# The archive holds a komac that prints its version
mkdir "$tmp/archive"
printf '#!/bin/sh\necho "komac 2.0.0"\n' > "$tmp/archive/komac"
chmod 755 "$tmp/archive/komac"
tar -czf "$tmp/komac.tar.gz" -C "$tmp/archive" komac
sha256=$(sha256sum "$tmp/komac.tar.gz" | awk '{ print $1 }')

# The stub saves the archive for the expected URL, at the file -o names
mkdir "$tmp/bin"
cat > "$tmp/bin/curl" <<'STUB'
#!/bin/sh
out=
url=
while [ $# -gt 0 ]; do
	case $1 in
	-o) out=$2; shift ;;
	-*) ;;
	*) url=$1 ;;
	esac
	shift
done
if [ "$url" != https://github.com/russellbanks/Komac/releases/download/v2.0.0/komac-2.0.0-x86_64-unknown-linux-gnu.tar.gz ]; then
	echo "curl: (22) The requested URL returned error: 404 for $url" >&2
	exit 22
fi
cp "$ARCHIVE" "$out"
STUB
chmod 755 "$tmp/bin/curl"
export ARCHIVE="$tmp/komac.tar.gz"

install_komac() {
	PATH=$tmp/bin:$PATH KOMAC_VERSION=$1 KOMAC_SHA256=$2 "$root/scripts/install-komac.sh" "$3"
}

expect_output "the archive with KOMAC_SHA256 installs" "Installed komac 2.0.0 into $tmp/good" \
	install_komac v2.0.0 "$sha256" "$tmp/good"
expect_success "the folder holds only komac" test "$(ls "$tmp/good")" = komac
other=$(printf '%s' "$sha256" | tr 0-9a-f 1-9a-f0)
expect_error "an archive with another SHA-256 fails" "is $sha256, not KOMAC_SHA256 ($other)" \
	install_komac v2.0.0 "$other" "$tmp/bad"
expect_success "a failed check leaves no komac" test ! -e "$tmp/bad/komac"
expect_failure "a version that does not exist fails" install_komac v2.0.1 "$sha256" "$tmp/missing"
expect_error "an unset KOMAC_SHA256 fails" "set KOMAC_SHA256" \
	install_komac v2.0.0 "" "$tmp/unset"

finish
