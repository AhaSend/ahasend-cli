#!/bin/sh
# Downloads the assets of a published release into a folder: checksums.txt
# and every asset it lists, from <AHASEND_RELEASES_URL>/download/<tag>/, as
# the install scripts do. It fails when a download fails or an asset does not
# match checksums.txt. The WinGet, Chocolatey and npm jobs publish these
# files, so they use the release's own assets and never rebuild them.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <tag> <dir>" >&2
	exit 1
fi
tag=$1
dir=$2
base=${AHASEND_RELEASES_URL:-https://github.com/AhaSend/ahasend-cli/releases}/download/$tag

mkdir -p "$dir"
curl -fsSL --retry 3 -o "$dir/checksums.txt" "$base/checksums.txt"
if [ ! -s "$dir/checksums.txt" ]; then
	echo "fetch-release: the checksums.txt of $tag is empty" >&2
	exit 1
fi
awk '{ print $2 }' "$dir/checksums.txt" | while read -r name; do
	case $name in
	*/* | .*)
		echo "fetch-release: checksums.txt lists $name, which is not an asset name" >&2
		exit 1
		;;
	esac
	curl -fsSL --retry 3 -o "$dir/$name" "$base/$name"
done
if ! (cd "$dir" && sha256sum -c --quiet --strict checksums.txt >&2); then
	echo "fetch-release: an asset of $tag does not match its checksums.txt" >&2
	exit 1
fi
echo "Downloaded the $(wc -l < "$dir/checksums.txt" | tr -d ' ') assets of $tag into $dir"
