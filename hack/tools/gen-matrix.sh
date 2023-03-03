#!/usr/bin/env sh

# Finds the last k0s releases of the given versions and generates json MATRIX_OUTPUT for github actions.
# Usage:
#  ./gen-matrix.sh 1.24.2 1.24.3
# Output: ["v1.24.2+k0s.0", "v1.24.3+k0s.0"]

list_k0s_releases() {
  gh api -X GET /repos/k0sproject/k0s/releases \
    -F per_page=100 --paginate \
    --jq '.[] | select(.prerelease == false and .draft == false) | .name'
}

k0s_sort() {
  go run github.com/k0sproject/version/cmd/k0s_sort@v0.2.2
}

latest_release() {
  list_k0s_releases | grep -F "v$1" | k0s_sort | tail -1
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
