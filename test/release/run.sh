#!/bin/sh
# Runs the tests of the release scripts: every test/release/*_test.sh, or the
# test files given, each in its own shell from the repository root. It needs
# no network once the Go modules are downloaded.
set -eu

cd "$(dirname "$0")/../.."
if [ $# -eq 0 ]; then
	set -- test/release/*_test.sh
fi

failed=
for test in "$@"; do
	echo "== $test"
	sh "$test" || failed="$failed $test"
done
if [ -n "$failed" ]; then
	echo "Failed:$failed" >&2
	exit 1
fi
echo "All release script tests passed"
