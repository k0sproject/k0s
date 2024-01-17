#!/usr/bin/env bash

# Finds the last k0s releases of the given versions and generates json MATRIX_OUTPUT for github actions.
# Usage:
#  ./gen-matrix.sh 1.24.2 1.24.3
# Output: ["v1.24.2+k0s.0", "v1.24.3+k0s.0"]

set -euo pipefail

list_k0s_releases() {
  # shellcheck disable=SC2016
  local query='.[] | select(.prerelease == false and .draft == false) | .name | select(startswith($ENV.VERSION_PREFIX))'
  VERSION_PREFIX="v$1" gh api -X GET /repos/k0sproject/k0s/releases -F per_page=100 --paginate --jq "$query"
}

latest_release() {
  list_k0s_releases "$1" | k0s_sort -l
}

json_print_latest_releases() {
  printf '['

  pattern='"%s"'
  for i in "$@"; do
    latestRelease="$(latest_release "$i")"
    [ -z "$latestRelease" ] || {
      # shellcheck disable=SC2059
      printf "$pattern" "$latestRelease"
      pattern=', "%s"'
    }
  done

  echo ']'
}

json_print_latest_releases "$@"
