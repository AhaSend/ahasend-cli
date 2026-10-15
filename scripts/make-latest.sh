#!/bin/sh
# Prints true when a release tag is to be marked as GitHub's latest release,
# and false otherwise; see release-kind.sh. It always prints a value, because
# GitHub marks a release latest when it is given none.
set -eu

if [ $# -ne 1 ]; then
	echo "usage: $0 <tag>" >&2
	exit 1
fi

kind=$("$(dirname "$0")/release-kind.sh" "$1")
if [ "$kind" = latest ]; then
	echo true
else
	echo false
fi
