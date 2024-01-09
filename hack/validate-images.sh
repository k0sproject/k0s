#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: 2024 k0s authors
# shellcheck disable=SC3043

set -eu

check_platforms() {
  # shellcheck disable=SC2016
  local filter='
    $ARGS.positional - [.manifests[].platform | "\(.os)/\(.architecture)"]
    | if length == 0 then "ok" else "missing platforms: " + join(", ") end
  '
  docker manifest inspect -- "$image" | jq --args -r "$filter" -- "$@"
}

ret=0
while read -r image; do
  case "$image" in
  */envoy-distroless:*) set -- linux/amd64 linux/arm64 ;;
  *) set -- linux/amd64 linux/arm64 linux/arm ;;
  esac

  printf 'Checking image validity for %s ... ' "$image" >&2
  check=$(check_platforms "$@")
  if [ "$check" != "ok" ]; then
    ret=1
    printf '\033[31m%s\033[0m\n' "$check" >&2
  else
    printf '\033[32mok: %s\033[0m\n' "$*" >&2
  fi
done

exit $ret
