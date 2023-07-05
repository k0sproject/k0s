#!/usr/bin/env sh

set -eu

print_usage() {
  cat <<EOF
Usage: $0

Creates and pushes a k0s release tag by performing the following steps:

  - Determine the Kubernetes version
  - Ask about RC and hotfix numbers
  - Construct the tag name from the above information
  - Create the tag and push it to the remote repository

OPTIONS:
    -h      Show this message

ENVIRONMENT:
    K0S_SIGNED_TAG   Used to optionally disable tag signing if set to
                     'no'.
EOF
}

confirm_action() {
  printf '%s (Y/n) ' "$1"

  while :; do
    read -r yn
    if [ -n "$yn" ]; then
      case "$yn" in
      y | Y) return 0 ;;
      n | N) return 1 ;;
      *) printf "To confirm, enter 'y', 'Y' or nothing at all, to decline, enter 'n' or 'N': " ;;
      esac
    else
      return 0
    fi
  done
}

git_get_upstream_remote_for_local_branch() {
  baseBranchFullRef=$(git rev-parse --symbolic-full-name "$1")
  upstreamRemoteRef=$(git for-each-ref --format='%(upstream)' "$baseBranchFullRef")

  case "$upstreamRemoteRef" in
  refs/remotes/*)
    upstreamRemoteBranch=${upstreamRemoteRef##"refs/remotes/"}
    echo "${upstreamRemoteBranch%/*}"
    ;;
  esac
}

git_get_current_branch_name() {
  git rev-parse --symbolic-full-name --abbrev-ref HEAD
}

determine_k8s_version() {
  printf %s 'Kubernetes version:  '
  set -- ./vars.sh kubernetes_version
  k8sVersion=$("$@" 2>/dev/null) || {
    retVal=$?
    echo Failed to determine Kubernetes version! 1>&2
    "$@"
    return $retVal
  }

  echo "$k8sVersion"
}

determine_git_base_branch() {
  printf %s 'Base git branch:     '
  baseBranchName=$(git_get_current_branch_name)
  echo "$baseBranchName"
}

determine_upstream_git_remote() {
  printf %s 'Upstream git remote: '
  upstreamRemote=$(git_get_upstream_remote_for_local_branch "$baseBranchName")
  if [ -n "$upstreamRemote" ]; then
    echo "$upstreamRemote"
  else
    echo N/A
  fi
}

read_k0s_rc_version() {
  if confirm_action 'Is the next version a release candidate?'; then
    while :; do
      printf %s 'Please enter the release candidate number (e.g. 0, 1, 2, ...): '
      read -r k0sRc
      ! [ "$k0sRc" -ge 0 ] 2>/dev/null || return 0
    done
  else
    k0sRc=''
  fi
}

read_k0s_hotfix_version() {
  while :; do
    printf %s 'Please enter the k0s hotfix number (e.g. 0, 1, 2, ...): '
    read -r k0sHotfix
    ! [ "$k0sHotfix" -ge 0 ] 2>/dev/null || return 0
  done
}

construct_tag_name() {
  k0sTag="v$k8sVersion"
  [ -z "${k0sRc-}" ] || k0sTag="$k0sTag-rc.$k0sRc"
  k0sTag="$k0sTag+k0s.$k0sHotfix"
}

create_tag() {
  set -- git tag -a -m "release $k0sTag"
  if [ "${K0S_SIGNED_TAG-}" != no ]; then
    set -- "$@" --sign
    confirm_action "About to create signed tag '$k0sTag' (disable with K0S_SIGNED_TAG=no). Okay?"
  else
    set -- "$@" --no-sign
    confirm_action "About to create unsigned tag '$k0sTag'. Okay?"
  fi

  "$@" -- "$k0sTag"
}

push_tag_to_upstream_remote() {
  git show "$k0sTag"
  if [ -n "$upstreamRemote" ]; then
    if confirm_action "Push the above tag '$k0sTag' to remote '$upstreamRemote'?"; then
      git push "$upstreamRemote" "$k0sTag"
    fi
  fi
}

do_release() {
  determine_k8s_version
  determine_git_base_branch
  determine_upstream_git_remote
  read_k0s_rc_version
  read_k0s_hotfix_version
  construct_tag_name
  create_tag
  push_tag_to_upstream_remote
}

if [ $# = 1 ] && [ "$1" = "-h" ]; then
  print_usage
  exit 1
fi

if [ $# != 0 ]; then
  print_usage >&2
  exit 1
fi

do_release
