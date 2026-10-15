#!/bin/sh
# Prints the build information that the version stamp of every build takes,
# as NAME=value lines: GIT_COMMIT, the first 7 characters of the commit
# hash, and BUILD_TIME, the commit time in UTC in RFC 3339. Both come from the
# commit, so every build of one commit prints the same version.
set -eu

commit=$(git rev-parse HEAD)
time=$(TZ=UTC git log -1 --format=%cd --date=format-local:%Y-%m-%dT%H:%M:%SZ)
echo "GIT_COMMIT=$(echo "$commit" | cut -c1-7)"
echo "BUILD_TIME=$time"
