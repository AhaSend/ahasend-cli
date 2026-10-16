# shellcheck shell=sh
# Helpers for the release script tests, which source this file. A test file
# runs its checks with expect_output, expect_success, expect_failure and
# expect_error, and ends with finish, which fails when any check failed.

root=$(pwd)
# shellcheck disable=SC2034 # used by the test files
fixtures=$root/test/release/fixtures
failures=0
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

# expect_output NAME EXPECTED COMMAND... passes when COMMAND succeeds and
# prints EXPECTED
expect_output() {
	name=$1
	expected=$2
	shift 2
	if actual=$("$@" 2> "$tmp/stderr"); then
		if [ "$actual" = "$expected" ]; then
			echo "ok: $name"
			return
		fi
		echo "FAIL: $name: expected:"
		printf '%s\n' "$expected"
		echo "got:"
		printf '%s\n' "$actual"
	else
		echo "FAIL: $name: the command failed:"
		cat "$tmp/stderr"
	fi
	failures=$((failures + 1))
}

# expect_success NAME COMMAND... passes when COMMAND succeeds
expect_success() {
	name=$1
	shift
	if "$@" > "$tmp/stdout" 2>&1; then
		echo "ok: $name"
	else
		echo "FAIL: $name: the command failed:"
		cat "$tmp/stdout"
		failures=$((failures + 1))
	fi
}

# expect_failure NAME COMMAND... passes when COMMAND fails
expect_failure() {
	name=$1
	shift
	if "$@" > "$tmp/stdout" 2>&1; then
		echo "FAIL: $name: the command succeeded:"
		cat "$tmp/stdout"
		failures=$((failures + 1))
	else
		echo "ok: $name"
	fi
}

# expect_error NAME TEXT COMMAND... passes when COMMAND fails with an output
# that holds TEXT
expect_error() {
	name=$1
	text=$2
	shift 2
	if "$@" > "$tmp/stdout" 2>&1; then
		echo "FAIL: $name: the command succeeded:"
	elif grep -qF -- "$text" "$tmp/stdout"; then
		echo "ok: $name"
		return
	else
		echo "FAIL: $name: the output does not hold \"$text\":"
	fi
	cat "$tmp/stdout"
	failures=$((failures + 1))
}

# scratch_repo DIR TAG... creates a git repository in DIR with one commit and
# the given tags on it
scratch_repo() {
	dir=$1
	shift
	git init -q "$dir"
	git -C "$dir" -c user.name=test -c user.email=test@example.com -c commit.gpgsign=false \
		commit -q --allow-empty -m test
	for tag in "$@"; do
		git -C "$dir" tag "$tag"
	done
}

# bare_repo DIR BRANCH [SRC DST]... creates a bare git repository in DIR,
# which stands in for a channel's repository. Given files, it has one commit
# on BRANCH with each file SRC at the path DST; without, it is empty.
bare_repo() {
	dir=$1
	branch=$2
	shift 2
	git init -q --bare "$dir"
	if [ $# -eq 0 ]; then
		return
	fi
	git init -q "$dir.work"
	git -C "$dir.work" checkout -q -b "$branch"
	while [ $# -gt 0 ]; do
		mkdir -p "$(dirname "$dir.work/$2")"
		cp "$1" "$dir.work/$2"
		shift 2
	done
	git -C "$dir.work" add -A
	git -C "$dir.work" -c user.name=test -c user.email=test@example.com -c commit.gpgsign=false \
		commit -q -m test
	git -C "$dir.work" push -q "$dir" "$branch"
}

# skip_unless VARIABLE WHY ends the test file, printing why it is skipped,
# unless VARIABLE is set. The tests that need the network or Verdaccio use it,
# so make test-scripts stays offline; packaging.yml sets the variables.
skip_unless() {
	if eval "[ -z \"\${$1:-}\" ]"; then
		echo "skip: $2; set $1 to run it"
		exit 0
	fi
}

finish() {
	if [ "$failures" -ne 0 ]; then
		echo "$failures check(s) failed"
		exit 1
	fi
}
