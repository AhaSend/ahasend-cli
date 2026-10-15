#!/bin/sh
# Tests install.sh against a fake release: the staged assets of the test
# snapshots v99.0.0 and v98.0.0-rc.1 in <release>/<tag>/, served by the
# fake-release server (test/install/fake-release):
#
#   test/install/run.sh <release> <fake-release>
#
# TEST_SHELL is the shell that runs install.sh: sh by default, or for example
# dash or "busybox sh". Every install runs with a new temporary HOME and
# TMPDIR, and with a PATH that holds only the tools the script needs. The test
# of --install-dir /usr/bin runs only in CI, where it uses sudo to create and
# remove /usr/bin/ahasend. It needs no network.
set -eu

if [ $# -ne 2 ]; then
	echo "usage: $0 <release> <fake-release>" >&2
	exit 1
fi
release=$(cd "$1" && pwd)
server=$(cd "$(dirname "$2")" && pwd)/$(basename "$2")
cd "$(dirname "$0")/../.."
# shellcheck source=test/release/lib.sh
. test/release/lib.sh

# The developer's settings must not change where or what the script installs
unset XDG_BIN_HOME AHASEND_VERSION AHASEND_INSTALL_DIR AHASEND_RELEASES_URL NO_COLOR

# The shell that runs install.sh, with a full path, so the test PATH can run it
set -f
# shellcheck disable=SC2086 # split into the command and its arguments
set -- ${TEST_SHELL:-sh}
set +f
shell=$(command -v "$1")
shift
shell="$shell${*:+ $*}"
echo "Testing install.sh with $shell"

# The release asset of this machine, named as install.sh names it
case $(uname -s) in
Linux) host_os=linux ;;
Darwin) host_os=darwin ;;
esac
case $(uname -m) in
x86_64 | amd64) host_arch=amd64 ;;
aarch64 | arm64) host_arch=arm64 ;;
esac
if [ "$host_os" = darwin ] && [ "$(sysctl -n hw.optional.arm64 2> /dev/null || true)" = 1 ]; then
	host_arch=arm64
fi
asset=ahasend-$host_os-$host_arch
if [ "$host_arch" = amd64 ]; then
	rpm_arch=x86_64
else
	rpm_arch=aarch64
fi

# make_bin DIR TOOL... links the given tools, those this machine has, into DIR
make_bin() {
	dir=$1
	shift
	mkdir -p "$dir"
	for tool in "$@"; do
		if found=$(command -v "$tool"); then
			ln -s "$found" "$dir/$tool"
		elif [ -x "/usr/sbin/$tool" ]; then
			ln -s "/usr/sbin/$tool" "$dir/$tool"
		fi
	done
}
tools="uname mktemp mkdir cp mv rm chmod awk readlink sysctl"
# shellcheck disable=SC2086 # one word per tool
{
	make_bin "$tmp/bin" $tools curl wget sha256sum shasum openssl
	make_bin "$tmp/bin-no-curl" $tools wget sha256sum shasum openssl
	make_bin "$tmp/bin-no-sha256sum" $tools curl wget shasum openssl
	make_bin "$tmp/bin-no-shasum" $tools curl wget openssl
	make_bin "$tmp/bin-no-hasher" $tools curl wget
}

# An executable that stands in for an ahasend that something else installed
mkdir -p "$tmp/other"
printf '#!/bin/sh\necho "AhaSend CLI v0.0.1"\n' > "$tmp/other/ahasend"
chmod 755 "$tmp/other/ahasend"

# fake_command NAME LINE writes a command NAME into $tmp/fake-NAME/ that runs
# the shell code LINE
fake_command() {
	mkdir -p "$tmp/fake-$1"
	printf '#!/bin/sh\n%s\n' "$2" > "$tmp/fake-$1/$1"
	chmod 755 "$tmp/fake-$1/$1"
}
# shellcheck disable=SC2016 # the code of the fake commands
{
	fake_command dpkg 'echo "ahasend: $2"'
	fake_command rpm 'echo ahasend-0.0.1-1'
	fake_command pacman 'echo "$2 is owned by ahasend-bin 0.0.1-1"'
}

# Tags the overlay folder adds to the release, which the server serves first
overlay=$tmp/overlay
# v97.0.0: the binary does not match checksums.txt
mkdir -p "$overlay/v97.0.0"
cp "$release/v99.0.0/checksums.txt" "$release/v99.0.0/$asset" "$overlay/v97.0.0/"
printf x >> "$overlay/v97.0.0/$asset"
# v96.0.0: checksums.txt lists the binary, which is missing
mkdir -p "$overlay/v96.0.0"
cp "$release/v99.0.0/checksums.txt" "$overlay/v96.0.0/"
# v94.0.0: checksums.txt has no line for the binary
mkdir -p "$overlay/v94.0.0"
awk -v name="$asset" '$2 != name' "$release/v99.0.0/checksums.txt" > "$overlay/v94.0.0/checksums.txt"
cp "$release/v99.0.0/$asset" "$overlay/v94.0.0/"
# v93.0.0: the binary matches checksums.txt but does not run
mkdir -p "$overlay/v93.0.0"
printf '#!/bin/sh\nexit 1\n' > "$overlay/v93.0.0/$asset"
(
	cd "$overlay/v93.0.0"
	if command -v sha256sum > /dev/null 2>&1; then
		sha256sum "$asset"
	else
		shasum -a 256 "$asset"
	fi
) > "$overlay/v93.0.0/checksums.txt"
# The script under test, as the latest release serves it
mkdir -p "$overlay/v99.0.0"
cp install.sh "$overlay/v99.0.0/"
# v95.0.0, the server's -slow tag: downloads break off after three seconds
slow_tag=v95.0.0

echo v99.0.0 > "$tmp/latest"
"$server" -url "$tmp/url" -latest "$tmp/latest" -slow "$slow_tag" "$overlay" "$release" 2> "$tmp/server.log" &
server_pid=$!
trap 'kill "$server_pid" 2> /dev/null || true; rm -rf "$tmp"' EXIT
tries=0
while [ ! -s "$tmp/url" ]; do
	tries=$((tries + 1))
	if [ "$tries" -gt 100 ]; then
		echo "the fake release server did not start:" >&2
		cat "$tmp/server.log" >&2
		exit 1
	fi
	sleep 0.1
done
url=$(cat "$tmp/url")

homes=0
# new_home makes a new empty HOME, with an empty TMPDIR in it, for the next
# install, and sets the test PATH and the latest tag back to the usual ones
new_home() {
	homes=$((homes + 1))
	home=$tmp/home-$homes
	mkdir -p "$home/tmp"
	test_path=$tmp/bin
	echo v99.0.0 > "$tmp/latest"
}

# install_sh ARG... runs install.sh with the test's HOME, TMPDIR, PATH and
# fake release
install_sh() {
	# shellcheck disable=SC2086 # the shell and its arguments
	env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
		$shell "$root/install.sh" "$@"
}

# has_version DIR VERSION succeeds when DIR/ahasend prints VERSION, or, for
# VERSION none, when there is no DIR/ahasend
has_version() {
	if [ "$2" = none ]; then
		if [ -e "$1/ahasend" ]; then
			echo "$1/ahasend exists"
			return 1
		fi
		return 0
	fi
	actual=$("$1/ahasend" --version | sed -n 1p) || return
	if [ "$actual" != "AhaSend CLI $2" ]; then
		echo "$1/ahasend prints \"$actual\", not \"AhaSend CLI $2\""
		return 1
	fi
}

# no_leftovers [DIR] succeeds when TMPDIR is empty and DIR holds no
# temporary copy of the binary
no_leftovers() {
	if [ -n "$(ls -A "$home/tmp")" ]; then
		echo "files are left in TMPDIR:"
		ls -A "$home/tmp"
		return 1
	fi
	if [ $# -gt 0 ]; then
		for file in "$1"/.ahasend.*; do
			if [ -e "$file" ]; then
				echo "a temporary copy is left in $1: $file"
				return 1
			fi
		done
	fi
}

# installs DIR VERSION ARG... runs install.sh with ARG..., and succeeds when
# it succeeds, DIR/ahasend then prints VERSION, and no temporary file is left
installs() {
	dir=$1
	version=$2
	shift 2
	install_sh "$@" > "$tmp/install.out" 2>&1 || {
		cat "$tmp/install.out"
		echo "install.sh failed"
		return 1
	}
	cat "$tmp/install.out"
	has_version "$dir" "$version" && no_leftovers "$dir"
}

# fails TEXT DIR OLD ARG... runs install.sh with ARG..., and succeeds when
# it fails with an output that holds TEXT, DIR/ahasend still prints OLD (see
# has_version), and no temporary file is left
fails() {
	text=$1
	dir=$2
	old=$3
	shift 3
	if install_sh "$@" > "$tmp/install.out" 2>&1; then
		cat "$tmp/install.out"
		echo "install.sh succeeded"
		return 1
	fi
	cat "$tmp/install.out"
	if ! grep -qF -- "$text" "$tmp/install.out"; then
		echo "the output does not hold: $text"
		return 1
	fi
	has_version "$dir" "$old" && no_leftovers "$dir"
}

# output_has TEXT and output_lacks TEXT check the output of the last install
output_has() {
	grep -qF -- "$1" "$tmp/install.out" || {
		echo "the output does not hold: $1"
		return 1
	}
}
output_lacks() {
	if grep -qF -- "$1" "$tmp/install.out"; then
		echo "the output holds: $1"
		return 1
	fi
}

skip() {
	echo "skip: $1: $2"
}

# Fresh installs

fresh_default() {
	new_home
	installs "$home/.local/bin" v99.0.0 && output_has "Installed AhaSend CLI v99.0.0"
}
expect_success "a fresh install goes into ~/.local/bin" fresh_default

fresh_xdg() {
	new_home
	(
		export XDG_BIN_HOME="$home/xdg-bin"
		installs "$home/xdg-bin" v99.0.0
	)
}
expect_success "a fresh install goes into XDG_BIN_HOME when it is set" fresh_xdg

fresh_option() {
	new_home
	installs "$home/opt/bin" v99.0.0 --install-dir "$home/opt/bin"
}
expect_success "--install-dir chooses the directory, and creates it" fresh_option

fresh_option_equals() {
	new_home
	installs "$home/opt/bin" v99.0.0 --install-dir="$home/opt/bin"
}
expect_success "--install-dir=<dir> chooses the directory" fresh_option_equals

fresh_env() {
	new_home
	(
		export AHASEND_INSTALL_DIR="$home/env-bin"
		installs "$home/env-bin" v99.0.0
	)
}
expect_success "AHASEND_INSTALL_DIR chooses the directory" fresh_env

piped() {
	new_home
	# shellcheck disable=SC2086 # the shell and its arguments
	env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
		$shell -s -- --install-dir "$home/bin" < install.sh > "$tmp/install.out" 2>&1 || {
		cat "$tmp/install.out"
		return 1
	}
	has_version "$home/bin" v99.0.0 && no_leftovers "$home/bin"
}
expect_success "install.sh piped into sh -s -- <options> installs" piped

from_release() {
	new_home
	# shellcheck disable=SC2086 # the shell and its arguments
	curl -fsSL "$url/latest/download/install.sh" |
		env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
			$shell -s -- --install-dir "$home/bin" > "$tmp/install.out" 2>&1 || {
		cat "$tmp/install.out"
		return 1
	}
	has_version "$home/bin" v99.0.0
}
if command -v curl > /dev/null 2>&1; then
	expect_success "curl <releases>/latest/download/install.sh | sh installs" from_release
else
	skip "curl <releases>/latest/download/install.sh | sh installs" "no curl"
fi

# Versions

upgrade() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 &&
		installs "$home/.local/bin" v99.0.0
}
expect_success "an install over an older binary upgrades it" upgrade

pin_option() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1
}
expect_success "--version installs that tag, a pre-release too" pin_option

pin_option_equals() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version=v98.0.0-rc.1
}
expect_success "--version=<tag> installs that tag" pin_option_equals

pin_env() {
	new_home
	(
		export AHASEND_VERSION=v98.0.0-rc.1
		installs "$home/.local/bin" v98.0.0-rc.1
	)
}
expect_success "AHASEND_VERSION installs that tag" pin_env

prerelease_latest() {
	new_home
	installs "$home/.local/bin" v99.0.0 || return
	echo v98.0.0-rc.1 > "$tmp/latest"
	fails "is not a stable version" "$home/.local/bin" v99.0.0
}
expect_success "a pre-release returned as latest is refused, keeping the old binary" prerelease_latest

# Options

quiet() {
	new_home
	installs "$home/.local/bin" v99.0.0 --quiet || return
	if [ -s "$tmp/install.out" ]; then
		echo "--quiet printed something"
		return 1
	fi
}
expect_success "--quiet prints nothing on success, not even the PATH hint" quiet

quiet_error() {
	new_home
	fails "error: could not download" "$home/.local/bin" none --quiet --version v96.0.0
}
expect_success "--quiet still prints errors" quiet_error

help() {
	new_home
	install_sh --help > "$tmp/install.out" 2>&1 || return
	output_has "Usage: install.sh" && output_has "--install-dir" && has_version "$home/.local/bin" none
}
expect_success "--help prints the options and installs nothing" help

new_home
expect_error "an unknown option fails" "unknown option: --bogus" install_sh --bogus
expect_error "--version without a tag fails" "--version needs a value" install_sh --version
expect_error "an empty --version fails" "--version needs a value" install_sh --version ''
expect_error "an empty --version= fails" "--version needs a value" install_sh --version=
expect_error "an empty --install-dir fails" "--install-dir needs a value" install_sh --install-dir ''
expect_error "an --install-dir that starts with ~ fails" "use \$HOME instead" install_sh '--install-dir=~/bin'
expect_error "a --version that is not a tag fails" "1.2.3 is not a release tag" install_sh --version 1.2.3

# in_terminal COMMAND runs the shell command COMMAND with a terminal as its
# stdout and stderr, through script
in_terminal() {
	if [ "$(uname -s)" = Darwin ]; then
		script -q /dev/null sh -c "$1"
	else
		script -qec "$1" /dev/null
	fi
}
# colored ARG... runs install.sh with ARG... in a terminal, and succeeds when
# its output has color codes
colored() {
	new_home
	in_terminal "env HOME=$home TMPDIR=$home/tmp PATH=$test_path AHASEND_RELEASES_URL=$url TERM=xterm \
		$shell $root/install.sh $*" > "$tmp/install.out" 2>&1 || {
		cat "$tmp/install.out"
		return 2
	}
	grep -q "$(printf '\033')\[" "$tmp/install.out"
}
not_colored() {
	colored "$@" && {
		echo "the output has color codes"
		return 1
	}
	[ $? -eq 1 ]
}
nocolor_env() {
	(
		export NO_COLOR=1
		not_colored "$@"
	)
}
if command -v script > /dev/null 2>&1; then
	expect_success "on a terminal, the output has colors" colored
	expect_success "--no-color turns colors off" not_colored --no-color
	expect_success "NO_COLOR turns colors off" nocolor_env
else
	skip "--no-color turns colors off" "no script command to make a terminal"
fi

# PATH

path_hint() {
	new_home
	installs "$home/.local/bin" v99.0.0 &&
		output_has "$home/.local/bin is not on PATH" &&
		output_has "export PATH=\"$home/.local/bin:\$PATH\"" &&
		output_has "fish_add_path \"$home/.local/bin\""
}
expect_success "an install directory not on PATH gets the lines that add it" path_hint

no_path_hint() {
	new_home
	test_path=$tmp/bin:$home/.local/bin
	installs "$home/.local/bin" v99.0.0 && output_lacks "is not on PATH" && output_lacks "warning"
}
expect_success "an install directory on PATH gets no hint" no_path_hint

other_copy() {
	new_home
	test_path=$tmp/other:$tmp/bin:$home/.local/bin
	installs "$home/.local/bin" v99.0.0 &&
		output_has "warning: $tmp/other/ahasend comes first on PATH" &&
		output_has "Remove it, or put $home/.local/bin before $tmp/other on PATH" &&
		output_lacks "is not on PATH"
}
expect_success "another copy earlier on PATH installs and warns" other_copy

# brew_link DIR makes DIR/bin/ahasend a link into a fake Homebrew Cellar in DIR
brew_link() {
	mkdir -p "$1/Cellar/ahasend/0.0.1/bin" "$1/bin"
	cp "$tmp/other/ahasend" "$1/Cellar/ahasend/0.0.1/bin/"
	ln -s ../Cellar/ahasend/0.0.1/bin/ahasend "$1/bin/ahasend"
}

other_owned_copy() {
	new_home
	brew_link "$home/brew"
	test_path=$home/brew/bin:$tmp/bin
	installs "$home/.local/bin" v99.0.0 &&
		output_has "warning: $home/brew/bin/ahasend comes first on PATH" &&
		output_has "Homebrew installed it; upgrade it with: brew upgrade ahasend/tap/ahasend" &&
		output_has "is not on PATH"
}
expect_success "another copy that Homebrew installed earlier on PATH installs and warns" other_owned_copy

# Failures, each keeping the old binary

mismatch() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 &&
		fails "does not match checksums.txt" "$home/.local/bin" v98.0.0-rc.1 --version v97.0.0
}
expect_success "a checksum mismatch fails and keeps the old binary" mismatch

missing_asset() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 &&
		fails "could not download $url/download/v96.0.0/$asset" "$home/.local/bin" v98.0.0-rc.1 --version v96.0.0
}
expect_success "a missing asset fails and keeps the old binary" missing_asset

missing_checksum() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 &&
		fails "checksums.txt has no line for $asset" "$home/.local/bin" v98.0.0-rc.1 --version v94.0.0
}
expect_success "a checksums.txt without the binary fails and keeps the old binary" missing_checksum

broken_binary() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 &&
		fails "the downloaded binary does not run" "$home/.local/bin" v98.0.0-rc.1 --version v93.0.0
}
expect_success "a binary that does not run fails and keeps the old binary" broken_binary

no_latest() {
	new_home
	installs "$home/.local/bin" v99.0.0 || return
	rm "$tmp/latest"
	fails "could not find the latest release at $url/latest" "$home/.local/bin" v99.0.0
}
expect_success "a 404 on latest fails and keeps the old binary" no_latest

unwritable() {
	new_home
	installs "$home/ro" v98.0.0-rc.1 --version v98.0.0-rc.1 --install-dir "$home/ro" || return
	chmod 555 "$home/ro"
	fails "cannot write to $home/ro" "$home/ro" v98.0.0-rc.1 --install-dir "$home/ro"
	status=$?
	chmod 755 "$home/ro"
	return "$status"
}
if [ "$(id -u)" -ne 0 ]; then
	expect_success "a directory the script cannot write to fails and keeps the old binary" unwritable
else
	skip "a directory the script cannot write to fails" "root can write to any directory"
fi

# Package-manager installs

owned_brew() {
	new_home
	brew_link "$home/brew"
	fails "Upgrade it with: brew upgrade ahasend/tap/ahasend" "$home/brew/bin" v0.0.1 \
		--install-dir "$home/brew/bin" || return
	[ "$(readlink "$home/brew/bin/ahasend")" = ../Cellar/ahasend/0.0.1/bin/ahasend ]
}
expect_success "a link into a Homebrew Cellar is refused, naming brew upgrade" owned_brew

owned_npm() {
	new_home
	mkdir -p "$home/npm/lib/node_modules/@ahasend/cli/bin" "$home/npm/bin"
	cp "$tmp/other/ahasend" "$home/npm/lib/node_modules/@ahasend/cli/bin/ahasend.js"
	ln -s ../lib/node_modules/@ahasend/cli/bin/ahasend.js "$home/npm/bin/ahasend"
	fails "Upgrade it with: npm install -g @ahasend/cli" "$home/npm/bin" v0.0.1 --install-dir "$home/npm/bin"
}
expect_success "a link into node_modules is refused, naming npm install -g" owned_npm

owned_link() {
	new_home
	mkdir -p "$home/links"
	ln -s "$tmp/other/ahasend" "$home/links/ahasend"
	fails "Upgrade it with that package manager" "$home/links" v0.0.1 --install-dir "$home/links"
}
expect_success "any other link is refused with the general message" owned_link

# owned_by MANAGER TEXT puts an ahasend that a fake MANAGER claims in a new
# directory, and checks that install.sh refuses to replace it with TEXT
owned_by() {
	new_home
	mkdir -p "$home/system"
	cp "$tmp/other/ahasend" "$home/system/"
	test_path=$tmp/fake-$1:$tmp/bin
	fails "$2" "$home/system" v0.0.1 --install-dir "$home/system"
}
expect_success "a file that dpkg -S claims is refused, naming apt" \
	owned_by dpkg "Upgrade it with: sudo apt install ./ahasend_99.0.0_${host_arch}.deb"
expect_success "a file that rpm -qf claims is refused, naming rpm -U" \
	owned_by rpm "Upgrade it with: sudo rpm -U ./ahasend-99.0.0-1.$rpm_arch.rpm"
expect_success "a file that pacman -Qo claims is refused, naming the AUR package" \
	owned_by pacman "Upgrade it with: yay -S ahasend-bin"

# owned_through_link puts an ahasend that a fake dpkg claims by its real path
# only, and installs into a link to its directory and into a path with ..
owned_through_link() {
	new_home
	mkdir -p "$home/system"
	cp "$tmp/other/ahasend" "$home/system/"
	ln -s system "$home/link"
	claimed=$(cd -P "$home/system" && pwd -P)/ahasend
	mkdir -p "$home/fake"
	# shellcheck disable=SC2016 # the code of the fake command
	printf '#!/bin/sh\n[ "$2" = "%s" ] && echo "ahasend: $2"\n' "$claimed" > "$home/fake/dpkg"
	chmod 755 "$home/fake/dpkg"
	test_path=$home/fake:$tmp/bin
	fails "Upgrade it with: sudo apt install" "$home/system" v0.0.1 --install-dir "$home/link" &&
		fails "Upgrade it with: sudo apt install" "$home/system" v0.0.1 --install-dir "$home/link/../system"
}
expect_success "a claimed file reached through a link or .. is refused" owned_through_link

usr_bin() {
	new_home
	sudo cp "$tmp/other/ahasend" /usr/bin/ahasend
	fails "Upgrade it with that package manager" /usr/bin v0.0.1 --install-dir /usr/bin
	status=$?
	sudo rm -f /usr/bin/ahasend
	return "$status"
}
if [ -n "${CI:-}" ] && [ "$host_os" = linux ] && [ ! -e /usr/bin/ahasend ] && sudo -n true 2> /dev/null; then
	expect_success "an ahasend in /usr/bin is refused with the general message" usr_bin
else
	skip "an ahasend in /usr/bin is refused" "runs only in CI on Linux, with sudo"
fi

# Systems

unsupported_system() {
	new_home
	# shellcheck disable=SC2016 # the code of the fake command
	fake_command uname 'if [ "$1" = -s ]; then echo FreeBSD; else echo x86_64; fi'
	test_path=$tmp/fake-uname:$tmp/bin
	fails "Download the CLI for your system from $url" "$home/.local/bin" none
}
expect_success "an unsupported system fails, naming the releases page" unsupported_system

unsupported_machine() {
	new_home
	# shellcheck disable=SC2016 # the code of the fake command
	fake_command uname 'if [ "$1" = -s ]; then echo Linux; else echo armv7l; fi'
	test_path=$tmp/fake-uname:$tmp/bin
	fails "supports amd64 and arm64 machines, not armv7l" "$home/.local/bin" none
}
expect_success "an unsupported machine fails" unsupported_machine

rosetta() {
	new_home
	env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
		/usr/bin/arch -x86_64 /bin/sh "$root/install.sh" > "$tmp/install.out" 2>&1 || {
		cat "$tmp/install.out"
		return 1
	}
	cmp "$home/.local/bin/ahasend" "$release/v99.0.0/ahasend-darwin-arm64"
}
if [ "$host_os" = darwin ] && [ "$host_arch" = arm64 ] && { [ -n "${CI:-}" ] || /usr/bin/arch -x86_64 /usr/bin/true 2> /dev/null; }; then
	expect_success "under Rosetta, install.sh installs the arm64 binary" rosetta
else
	skip "under Rosetta, install.sh installs the arm64 binary" "needs an Apple silicon Mac with Rosetta"
fi

# Tools

with_bin() {
	new_home
	test_path=$tmp/$1
	shift
	installs "$home/.local/bin" "$@"
}
with_bin_refused() {
	new_home
	test_path=$tmp/$1
	installs "$home/.local/bin" v99.0.0 || return
	echo v98.0.0-rc.1 > "$tmp/latest"
	fails "is not a stable version" "$home/.local/bin" v99.0.0
}
if [ -e "$tmp/bin-no-curl/wget" ]; then
	expect_success "with wget and no curl, the latest release installs" with_bin bin-no-curl v99.0.0
	expect_success "with wget and no curl, a pinned release installs" \
		with_bin bin-no-curl v98.0.0-rc.1 --version v98.0.0-rc.1
	expect_success "with wget and no curl, a pre-release as latest is refused" with_bin_refused bin-no-curl
else
	skip "with wget and no curl, install.sh installs" "no wget"
fi
if [ -e "$tmp/bin-no-sha256sum/shasum" ]; then
	expect_success "without sha256sum, shasum checks the download" with_bin bin-no-sha256sum v99.0.0
else
	skip "without sha256sum, shasum checks the download" "no shasum"
fi
if [ -e "$tmp/bin-no-shasum/openssl" ]; then
	expect_success "without sha256sum and shasum, openssl checks the download" with_bin bin-no-shasum v99.0.0
else
	skip "without sha256sum and shasum, openssl checks the download" "no openssl"
fi
no_hasher() {
	new_home
	test_path=$tmp/bin-no-hasher
	fails "needs sha256sum, shasum or openssl" "$home/.local/bin" none
}
expect_success "without any hashing tool, install.sh fails" no_hasher

# Interruptions

# cut_off checks that no start of install.sh that ends at a line boundary,
# piped into the shell as curl would, installs or downloads anything
cut_off() {
	new_home
	if [ "$(tail -n 1 install.sh)" != '{ main "$@"; }' ]; then
		echo 'the last line of install.sh is not: { main "$@"; }'
		return 1
	fi
	requests=$(wc -l < "$tmp/server.log")
	lines=$(wc -l < install.sh)
	n=1
	while [ "$n" -lt "$lines" ]; do
		head -n "$n" install.sh > "$tmp/cut.sh"
		# shellcheck disable=SC2086 # the shell and its arguments
		env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
			$shell -s < "$tmp/cut.sh" > /dev/null 2>&1 || true
		n=$((n + 1))
	done
	if [ -e "$home/.local" ]; then
		echo "a cut-off script created $home/.local"
		return 1
	fi
	if [ "$(wc -l < "$tmp/server.log")" -ne "$requests" ]; then
		echo "a cut-off script downloaded something"
		return 1
	fi
	no_leftovers
}
expect_success "a script cut off at any line runs nothing" cut_off

# stopped_by SIGNAL STATUS starts a download of the slow tag over an older
# binary, sends SIGNAL to the script once the download starts, and checks
# that the script exits with STATUS and keeps the old binary, with no
# temporary file left
stopped_by() {
	new_home
	installs "$home/.local/bin" v98.0.0-rc.1 --version v98.0.0-rc.1 > /dev/null || return
	rm -f "$tmp/pid"
	before=$(grep -c "/download/$slow_tag/" "$tmp/server.log" || true)
	# Runs in the background: waits for the download, then signals the script,
	# which runs in the foreground, so its SIGINT is not ignored
	(
		tries=0
		while [ ! -s "$tmp/pid" ] || [ "$(grep -c "/download/$slow_tag/" "$tmp/server.log" || true)" -le "$before" ]; do
			tries=$((tries + 1))
			if [ "$tries" -gt 100 ]; then
				exit 1
			fi
			sleep 0.1
		done
		kill -s "$1" "$(cat "$tmp/pid")"
	) &
	watcher=$!
	status=0
	# shellcheck disable=SC2016,SC2086 # $$ is the script's process; the shell and its arguments
	env HOME="$home" TMPDIR="$home/tmp" PATH="$test_path" AHASEND_RELEASES_URL="$url" \
		$shell -c 'echo $$ > "$0"; exec "$@"' "$tmp/pid" $shell "$root/install.sh" --version "$slow_tag" \
		> "$tmp/install.out" 2>&1 || status=$?
	if ! wait "$watcher"; then
		echo "the download did not start"
		return 1
	fi
	if [ "$status" -ne "$2" ]; then
		cat "$tmp/install.out"
		echo "install.sh exited with $status, not $2"
		return 1
	fi
	has_version "$home/.local/bin" v98.0.0-rc.1 && no_leftovers "$home/.local/bin"
}
expect_success "SIGINT during the download stops the script and leaves no temporary file" stopped_by INT 130
expect_success "SIGTERM during the download stops the script and leaves no temporary file" stopped_by TERM 143
expect_success "SIGHUP during the download stops the script and leaves no temporary file" stopped_by HUP 129

finish
