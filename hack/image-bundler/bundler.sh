#!/usr/bin/env sh
#shellcheck shell=ash

set -euo pipefail

containerd </dev/null >&2 &
#shellcheck disable=SC2064
trap "{ kill -- $! && wait -- $!; } || true" INT EXIT

while ! ctr version </dev/null >/dev/null; do
  kill -0 $!
  echo containerd not yet available >&2
  sleep 1
done

echo containerd up >&2

set --

while read -r image; do
  echo Fetching content of "$image" ... >&2
  out="$(ctr content fetch --platform "$TARGET_PLATFORM" -- "$image")" || {
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

out="${TMPDIR-/tmp}/bundle.tar"
echo Exporting images ... >&2
set +x
ctr images export --platform "$TARGET_PLATFORM" -- - "$@" >"$out"
sha256sum -- "$out" >&2
stat -- "$out" >&2
tar tf "$out" >/dev/null
cat -- "$out"
echo Images exported. >&2
