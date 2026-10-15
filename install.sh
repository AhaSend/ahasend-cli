#!/bin/sh
# Installs the AhaSend CLI on macOS and Linux:
#
#   curl -fsSL https://ahasend.com/install.sh | sh
#
# Options go after "sh -s --", for example:
#
#   curl -fsSL https://ahasend.com/install.sh | sh -s -- --version v0.4.0
#
# It downloads the release binary for this system, checks it against the
# release's checksums.txt, and installs it in ~/.local/bin. It never runs
# sudo, never edits shell startup files, and never replaces a file that a
# package manager installed. Run it with --help to see the options.
#
# All code is in functions and the last line runs them, so a download that
# stops halfway runs nothing.
set -eu

default_releases_url=https://github.com/AhaSend/ahasend-cli/releases
stable_tag='^v[0-9]+[.][0-9]+[.][0-9]+$'
any_tag='^v[0-9]+[.][0-9]+[.][0-9]+(-[0-9A-Za-z.-]+)?$'

usage() {
	printf '%s\n' \
		'Usage: install.sh [options]' \
		'' \
		'Installs the AhaSend CLI from its GitHub release.' \
		'' \
		'Options:' \
		'  --version <tag>      install this release, such as v0.4.0 (default: the latest)' \
		'  --install-dir <dir>  install into this directory' \
		"                       (default: \$XDG_BIN_HOME, or ~/.local/bin)" \
		'  --quiet              print only errors' \
		'  --no-color           print without colors' \
		'  --help               show this help' \
		'' \
		'Environment variables:' \
		'  AHASEND_VERSION       the same as --version' \
		'  AHASEND_INSTALL_DIR   the same as --install-dir' \
		'  AHASEND_RELEASES_URL  download from this copy of the releases page' \
		"                        (default: $default_releases_url)"
}

# say prints lines, unless --quiet is given
say() {
	if [ "$quiet" = false ]; then
		printf '%s\n' "$@"
	fi
}

# success prints a line in green, unless --quiet is given
success() {
	if [ "$quiet" = false ]; then
		printf '%s%s%s\n' "$green" "$1" "$out_reset"
	fi
}

# warn prints a warning and more lines to stderr, unless --quiet is given
warn() {
	if [ "$quiet" = false ]; then
		printf '%swarning:%s %s\n' "$yellow" "$err_reset" "$1" >&2
		shift
		printf '%s\n' "$@" >&2
	fi
}

# fail prints an error and more lines to stderr, and exits
fail() {
	printf '%serror:%s %s\n' "$red" "$err_reset" "$1" >&2
	shift
	if [ $# -gt 0 ]; then
		printf '%s\n' "$@" >&2
	fi
	exit 1
}

usage_error() {
	fail "$1" "Run install.sh --help to see the options."
}

parse_args() {
	version=${AHASEND_VERSION:-}
	install_dir=${AHASEND_INSTALL_DIR:-}
	quiet=false
	color=true
	while [ $# -gt 0 ]; do
		case $1 in
		--version | --install-dir)
			# An empty value, such as from an unset variable, would
			# otherwise mean the latest release or the default directory
			if [ $# -lt 2 ] || [ -z "$2" ]; then
				usage_error "$1 needs a value."
			fi
			if [ "$1" = --version ]; then
				version=$2
			else
				install_dir=$2
			fi
			shift 2
			;;
		--version=* | --install-dir=*)
			if [ -z "${1#*=}" ]; then
				usage_error "${1%%=*} needs a value."
			fi
			if [ "${1%%=*}" = --version ]; then
				version=${1#*=}
			else
				install_dir=${1#*=}
			fi
			shift
			;;
		--quiet)
			quiet=true
			shift
			;;
		--no-color)
			color=false
			shift
			;;
		--help | -h)
			usage
			exit 0
			;;
		*)
			usage_error "unknown option: $1"
			;;
		esac
	done
}

# set_colors colors the output on a terminal, unless --no-color or NO_COLOR
# turns colors off
set_colors() {
	if [ "$color" = false ] || [ -n "${NO_COLOR:-}" ] || [ "${TERM:-}" = dumb ]; then
		return
	fi
	esc=$(printf '\033')
	if [ -t 1 ]; then
		green="${esc}[32m"
		out_reset="${esc}[0m"
	fi
	if [ -t 2 ]; then
		red="${esc}[31m"
		yellow="${esc}[33m"
		err_reset="${esc}[0m"
	fi
}

# detect_platform sets os and arch to the names the release assets use
detect_platform() {
	system=$(uname -s)
	case $system in
	Linux) os=linux ;;
	Darwin) os=darwin ;;
	*) fail "this script supports macOS and Linux, not $system." \
		"Download the CLI for your system from $releases_url" ;;
	esac
	machine=$(uname -m)
	case $machine in
	x86_64 | amd64) arch=amd64 ;;
	aarch64 | arm64) arch=arm64 ;;
	*) fail "this script supports amd64 and arm64 machines, not $machine." \
		"Download the CLI for your system from $releases_url" ;;
	esac
	# Under Rosetta, uname -m says x86_64 on an Apple silicon Mac
	if [ "$os" = darwin ] && [ "$(sysctl -n hw.optional.arm64 2> /dev/null || true)" = 1 ]; then
		arch=arm64
	fi
}

# find_tools sets downloader and hasher to the tools this system has
find_tools() {
	if command -v curl > /dev/null 2>&1; then
		downloader=curl
	elif command -v wget > /dev/null 2>&1; then
		downloader=wget
	else
		fail "this script needs curl or wget."
	fi
	if command -v sha256sum > /dev/null 2>&1; then
		hasher=sha256sum
	elif command -v shasum > /dev/null 2>&1; then
		hasher=shasum
	elif command -v openssl > /dev/null 2>&1; then
		hasher=openssl
	else
		fail "this script needs sha256sum, shasum or openssl to check the download."
	fi
}

# download URL FILE saves URL in FILE
download() {
	if [ "$downloader" = curl ]; then
		curl -fsSL -o "$2" "$1"
	else
		wget -q -O "$2" "$1"
	fi
}

# sha256 FILE prints the SHA-256 of FILE
sha256() {
	case $hasher in
	sha256sum) sha256sum "$1" | awk '{ print $1 }' ;;
	shasum) shasum -a 256 "$1" | awk '{ print $1 }' ;;
	openssl) openssl dgst -sha256 "$1" | awk '{ print $NF }' ;;
	esac
}

# matches TEXT PATTERN succeeds when TEXT is one line that matches the
# extended regular expression PATTERN
matches() {
	printf '%s\n' "$1" | awk -v pattern="$2" '$0 ~ pattern { found = 1 } END { exit !(found && NR == 1) }'
}

# find_latest_tag sets tag to the tag of the newest stable release, from the
# URL that <releases>/latest redirects to. It does not use the GitHub API,
# which has rate limits.
find_latest_tag() {
	url=$releases_url/latest
	if [ "$downloader" = curl ]; then
		headers=$(curl -fsSI "$url") || fail "could not find the latest release at $url"
	else
		headers=$(wget -S --spider "$url" 2>&1) || fail "could not find the latest release at $url"
	fi
	location=$(printf '%s\n' "$headers" |
		awk 'tolower($1) == "location:" { location = $2 } END { sub(/\r$/, "", location); print location }')
	if [ -z "$location" ]; then
		fail "$url does not redirect to a release."
	fi
	tag=${location##*/tag/}
	if ! matches "$tag" "$stable_tag"; then
		fail "the latest release, $tag, is not a stable version." \
			"Choose a release with --version, from $releases_url"
	fi
}

# owner FILE succeeds when a package manager installed FILE. It sets manager
# to that manager's name and upgrade to the command that upgrades FILE, or
# both to nothing when it is not clear which manager it is.
owner() {
	manager=
	upgrade=
	if [ -L "$1" ]; then
		case $(readlink "$1" || true) in
		*/Cellar/*)
			manager=Homebrew
			upgrade="brew upgrade ahasend/tap/ahasend"
			;;
		*/node_modules/*)
			manager=npm
			upgrade="npm install -g @ahasend/cli"
			;;
		esac
		return 0
	fi
	if command -v dpkg > /dev/null 2>&1 && dpkg -S "$1" > /dev/null 2>&1; then
		manager=dpkg
		package=ahasend_${tag#v}_$arch.deb
		upgrade="sudo apt install ./$package, with $package from $releases_url/download/$tag/"
		return 0
	fi
	if command -v rpm > /dev/null 2>&1 && rpm -qf "$1" > /dev/null 2>&1; then
		manager=rpm
		if [ "$arch" = amd64 ]; then
			package=ahasend-${tag#v}-1.x86_64.rpm
		else
			package=ahasend-${tag#v}-1.aarch64.rpm
		fi
		upgrade="sudo rpm -U ./$package, with $package from $releases_url/download/$tag/"
		return 0
	fi
	if command -v pacman > /dev/null 2>&1 && pacman -Qo "$1" > /dev/null 2>&1; then
		manager=pacman
		upgrade="yay -S ahasend-bin (or the AUR helper you use)"
		return 0
	fi
	case $1 in
	/usr/bin/* | /nix/store/*) return 0 ;;
	esac
	return 1
}

# refuse_owned FILE fails when FILE exists and a package manager installed
# it, or when it is a directory
refuse_owned() {
	if [ -e "$1" ] || [ -L "$1" ]; then
		if owner "$1"; then
			if [ -n "$manager" ]; then
				fail "$manager installed $1, and this script does not replace it." \
					"Upgrade it with: $upgrade"
			fi
			fail "a package manager seems to have installed $1, and this script does not replace it." \
				"Upgrade it with that package manager, or choose another directory with --install-dir."
		fi
		if [ -d "$1" ]; then
			fail "$1 is a directory."
		fi
	fi
}

# prepare_install_dir sets install_dir and creates it, and fails when the
# script must not or cannot write ahasend into it
prepare_install_dir() {
	if [ -z "$install_dir" ]; then
		if [ -n "${XDG_BIN_HOME:-}" ]; then
			install_dir=$XDG_BIN_HOME
		elif [ -n "${HOME:-}" ]; then
			install_dir=$HOME/.local/bin
		else
			fail "HOME is not set; choose a directory with --install-dir."
		fi
	fi
	# The shell does not expand ~ after --install-dir=
	# shellcheck disable=SC2088 # a literal ~ is what this matches
	case $install_dir in
	'~' | '~/'*) usage_error "--install-dir $install_dir starts with ~; use \$HOME instead." ;;
	esac
	case $install_dir in
	/*) ;;
	*) install_dir=$(pwd)/$install_dir ;;
	esac
	if [ "$install_dir" != / ]; then
		install_dir=${install_dir%/}
	fi
	target=${install_dir%/}/ahasend

	# Checked first, so the message names the package manager even when the
	# directory is not writable. A package manager knows a file by its real
	# path, so a directory reached through a link (such as /bin on systems
	# where it links to /usr/bin) or with .. in it is checked by its real
	# path too.
	refuse_owned "$target"
	if [ -d "$install_dir" ]; then
		real_dir=$(CDPATH='' cd -P "$install_dir" 2> /dev/null && pwd -P) ||
			fail "cannot use $install_dir." "Choose another directory with --install-dir."
		real_target=${real_dir%/}/ahasend
		if [ "$real_target" != "$target" ]; then
			refuse_owned "$real_target"
		fi
	fi
	if [ ! -d "$install_dir" ]; then
		mkdir -p "$install_dir" 2> /dev/null ||
			fail "could not create $install_dir." "Choose another directory with --install-dir."
	fi
	if [ ! -w "$install_dir" ]; then
		fail "cannot write to $install_dir." \
			"This script never runs sudo. Choose a directory you can write to with --install-dir," \
			"such as --install-dir \"\$HOME/.local/bin\"."
	fi
	real_install_dir=$(cd "$install_dir" && pwd -P)
}

# install_binary downloads the binary, checks it and moves it into place.
# Until the last step, the old binary stays in place and works.
install_binary() {
	asset=ahasend-$os-$arch
	base=$releases_url/download/$tag
	say "Installing ahasend $tag ($os/$arch) into $install_dir"

	tmp_dir=$(mktemp -d)
	download "$base/checksums.txt" "$tmp_dir/checksums.txt" ||
		fail "could not download $base/checksums.txt"
	expected=$(awk -v name="$asset" '{ file = $2; sub(/^\*/, "", file) } file == name { print $1; exit }' \
		"$tmp_dir/checksums.txt")
	if [ -z "$expected" ]; then
		fail "$base/checksums.txt has no line for $asset."
	fi
	download "$base/$asset" "$tmp_dir/$asset" || fail "could not download $base/$asset"
	actual=$(sha256 "$tmp_dir/$asset")
	if [ "$actual" != "$expected" ]; then
		fail "the SHA-256 of $asset does not match checksums.txt, so the download is damaged." \
			"Expected $expected, got $actual."
	fi

	tmp_bin=$(mktemp "$install_dir/.ahasend.XXXXXX") || fail "cannot write to $install_dir."
	cp "$tmp_dir/$asset" "$tmp_bin"
	chmod 755 "$tmp_bin"
	version_output=$("$tmp_bin" --version 2>&1) || fail "the downloaded binary does not run:" "$version_output"
	mv -f "$tmp_bin" "$target"
	tmp_bin=
	success "Installed $(printf '%s\n' "$version_output" | awk 'NR == 1') at $target"
}

# check_path warns about another ahasend that comes first on PATH, and shows
# how to add the install directory to PATH when it is not on it
check_path() {
	on_path=false
	other=
	old_ifs=$IFS
	IFS=:
	set -f
	for dir in $PATH; do
		if [ -z "$dir" ]; then
			continue
		fi
		if [ "$(cd "$dir" 2> /dev/null && pwd -P)" = "$real_install_dir" ]; then
			on_path=true
			break
		fi
		if [ -z "$other" ] && [ -f "$dir/ahasend" ] && [ -x "$dir/ahasend" ]; then
			other=$dir/ahasend
		fi
	done
	set +f
	IFS=$old_ifs

	if [ -n "$other" ]; then
		if ! owner "$other"; then
			warn "$other comes first on PATH, so the ahasend command runs it, not $target." \
				"Remove it, or put $install_dir before ${other%/*} on PATH."
		elif [ -n "$manager" ]; then
			warn "$other comes first on PATH, so the ahasend command runs it, not $target." \
				"$manager installed it; upgrade it with: $upgrade"
		else
			warn "$other comes first on PATH, so the ahasend command runs it, not $target." \
				"A package manager seems to have installed it; upgrade or remove it with that package manager."
		fi
	fi
	if [ "$on_path" = false ]; then
		say "" \
			"$install_dir is not on PATH. To add it, put this line in your shell's startup file:" \
			"  sh, bash or zsh: export PATH=\"$install_dir:\$PATH\"" \
			"  fish:            fish_add_path \"$install_dir\""
	fi
}

cleanup() {
	if [ -n "$tmp_bin" ]; then
		rm -f "$tmp_bin"
	fi
	if [ -n "$tmp_dir" ]; then
		rm -rf "$tmp_dir"
	fi
}

main() {
	green=''
	out_reset=''
	red=''
	yellow=''
	err_reset=''
	tmp_dir=''
	tmp_bin=''
	trap cleanup EXIT
	trap 'exit 129' HUP
	trap 'exit 130' INT
	trap 'exit 143' TERM

	parse_args "$@"
	set_colors
	releases_url=${AHASEND_RELEASES_URL:-$default_releases_url}
	releases_url=${releases_url%/}
	detect_platform
	find_tools
	if [ -n "$version" ]; then
		if ! matches "$version" "$any_tag"; then
			usage_error "$version is not a release tag; tags look like v0.4.0."
		fi
		tag=$version
	else
		find_latest_tag
	fi
	prepare_install_dir
	install_binary
	check_path
}

# In braces, so a download cut off anywhere in this line is a syntax error
# and runs nothing
{ main "$@"; }
