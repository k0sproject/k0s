#!/usr/bin/env bash

set -eu -o pipefail

containerd </dev/null >&2 &
#shellcheck disable=SC2064

trap "{ kill -- $! && wait -- $!; } || true" INT EXIT

while ! ctr version </dev/null >/dev/null; do
  kill -0 $!
  echo containerd not yet available >&2
  sleep 1
done

echo containerd up >&2

set +u 

while read -r image; do
  if [[ ! -z $DOCKER_USER || ! -z $DOCKER_PASSWORD ]]; then
    auth="--user $DOCKER_USER:$DOCKER_PASSWORD"
  else
    auth=""
  fi
  echo Fetching content of "$image" ... >&2
  out="$(ctr content fetch --platform "$TARGET_PLATFORM" $auth -- "$image")" || {
    code=$?
    echo "$out" >&2
    exit $code
  }

  set -- "$@" "$image"
done

[ -n "$*" ] || {
  echo No images provided via STDIN! >&2
  exit 1
}

echo Exporting images ... >&2
ctr images export --platform "$TARGET_PLATFORM" -- - "$@"
echo Images exported. >&2
