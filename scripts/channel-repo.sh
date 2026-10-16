# shellcheck shell=sh
# Sourced by channel-published.sh and push-channel.sh. channel_repo CHANNEL
# sets, for the channels whose files live in a git repository:
#
#   repo_url     the repository; AHASEND_TAP_REPO, AHASEND_BUCKET_REPO or
#                AHASEND_AUR_REPO replaces it, so tests can use local repos
#   repo_branch  the branch users install from
#   repo_file    the file whose version is the version the channel has
#
# It fails for any other channel.

# shellcheck disable=SC2034 # used by the scripts that source this file
channel_repo() {
	case $1 in
	homebrew)
		repo_url=${AHASEND_TAP_REPO:-https://github.com/AhaSend/homebrew-tap.git}
		repo_branch=main
		repo_file=Formula/ahasend.rb
		;;
	scoop)
		repo_url=${AHASEND_BUCKET_REPO:-https://github.com/AhaSend/scoop-bucket.git}
		repo_branch=main
		repo_file=ahasend.json
		;;
	aur)
		repo_url=${AHASEND_AUR_REPO:-ssh://aur@aur.archlinux.org/ahasend-bin.git}
		# The AUR accepts pushes to master only
		repo_branch=master
		repo_file=.SRCINFO
		;;
	*)
		echo "$1 is not a channel with a git repository: use homebrew, scoop or aur" >&2
		exit 1
		;;
	esac
}
