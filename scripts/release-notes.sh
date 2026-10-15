#!/bin/sh
# Prints the release notes of a tag from CHANGELOG.md (or the given file):
# the section of a stable vX.Y.Z, under `## [X.Y.Z]` with or without a date,
# or the `## [Unreleased]` section for a pre-release. The section is printed
# without its heading, followed by the definitions of the reference-style
# links it uses. Links to files of the repository point at the files of the
# tag on GitHub, so they work on the release page. It fails when the section
# is missing or empty.
set -eu

if [ $# -lt 1 ] || [ $# -gt 2 ]; then
	echo "usage: $0 <tag> [<changelog>]" >&2
	exit 1
fi
tag=$1
changelog=${2:-CHANGELOG.md}

kind=$("$(dirname "$0")/release-kind.sh" "$tag")
if [ "$kind" = prerelease ]; then
	heading='## [Unreleased]'
else
	heading="## [${tag#v}]"
fi

# The section's lines without the heading, without blank lines at its start
# and end, and without link definitions, which the file keeps at its end
section=$(awk -v heading="$heading" '
	/^## / {
		if (found) exit
		if ($0 == heading || index($0, heading " - ") == 1) found = 1
		next
	}
	found && !/^\[[^]]+\]: / { print }
' "$changelog" | awk 'NF { started = 1 } started')
if [ -z "$section" ]; then
	echo "$changelog has no $heading section, or it is empty" >&2
	exit 1
fi

# The definitions of the links the section uses
definitions=$(grep '^\[[^]]*\]: ' "$changelog" | while IFS= read -r line; do
	label=${line%%]: *}]
	case "$section" in
	*"$label"*) printf '%s\n' "$line" ;;
	esac
done)

# Relative links, such as USAGE.mdx#exit-codes, become links to the tag's files
blob="https://github.com/AhaSend/ahasend-cli/blob/$tag"
{
	printf '%s\n' "$section"
	if [ -n "$definitions" ]; then
		printf '\n%s\n' "$definitions"
	fi
} | sed -e "s|](\([^):/#][^):]*\))|]($blob/\1)|g" \
	-e "s|^\(\[[^]]*\]: \)\([^:/#][^:]*\)\$|\1$blob/\2|"
