#!/bin/sh
# Tests scripts/channel-gates.sh in a repository with the tags v1.2.3, v1.2.4
# and v1.2.5, with a stub gh that knows a published release of v1.2.3, a
# draft of v1.2.4 and no release of v1.2.5.
set -eu
. test/release/lib.sh

scratch_repo "$tmp/repo" v1.2.3 v1.2.4 v1.2.5
cd "$tmp/repo"

# The stub answers `gh release view <tag> --json isDraft --jq .isDraft`
mkdir "$tmp/bin"
cat > "$tmp/bin/gh" <<'STUB'
#!/bin/sh
case "$*" in
"release view v1.2.3 --json isDraft --jq .isDraft") echo false ;;
"release view v1.2.4 --json isDraft --jq .isDraft") echo true ;;
*)
	echo "release not found" >&2
	exit 1
	;;
esac
STUB
chmod 755 "$tmp/bin/gh"
PATH=$tmp/bin:$PATH

expect_success "a published stable release on its tag passes" \
	"$root/scripts/channel-gates.sh" v1.2.3 refs/tags/v1.2.3
expect_error "a pre-release fails" "v1.2.6-rc.1 is a pre-release" \
	"$root/scripts/channel-gates.sh" v1.2.6-rc.1 refs/tags/v1.2.6-rc.1
expect_error "a branch ref fails" "the run's ref is refs/heads/main, not refs/tags/v1.2.3" \
	"$root/scripts/channel-gates.sh" v1.2.3 refs/heads/main
expect_error "another tag's ref fails" "the run's ref is refs/tags/v1.2.4, not refs/tags/v1.2.3" \
	"$root/scripts/channel-gates.sh" v1.2.3 refs/tags/v1.2.4
expect_error "a tag with no release fails" "v1.2.5 has no published GitHub release" \
	"$root/scripts/channel-gates.sh" v1.2.5 refs/tags/v1.2.5
expect_error "a draft release fails" "the GitHub release of v1.2.4 is a draft" \
	"$root/scripts/channel-gates.sh" v1.2.4 refs/tags/v1.2.4
expect_error "a tag that is not a release tag fails" "is not a release tag" \
	"$root/scripts/channel-gates.sh" v1.2 refs/tags/v1.2

finish
