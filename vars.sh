#!/usr/bin/env sh
#shellcheck disable=SC3043

set -eu

from=
var=

print_usage() {
  echo 'Query Makefile variables from scripts. Use like so:'
  echo
  echo "  $0 go_version"
  echo "  $0 FROM=docs python_version"
}

version_from_go_mod() {
  local pkg version rest
  while read -r pkg version rest; do
    if [ "$pkg" = "$1" ]; then
      printf %s "$version"
      return 0
    fi
  done

  return 1
}

fail() {
  echo "$@" >&2
  print_usage >&2
  exit 1
}

[ $# -gt 0 ] || { print_usage && exit; }

while [ $# -gt 0 ]; do
  case "$1" in
  *=*)
    name="${1%%=*}" && val="${1#*=}"
    [ "$name" = FROM ] || fail Unsupported variable "$name"
    [ -z "$from" ] || fail FROM already given
    [ -n "$val" ] || fail FROM may not be empty
    from="$val"
    ;;

  *)
    [ -z "$var" ] || fail Makefile variable already given
    var="$1"
    ;;
  esac
  shift 1
done

[ -n "$var" ] || fail Makefile variable not given
[ -n "$from" ] || from=embedded-bins

case "$var" in
k0sctl_version) pkg=github.com/k0sproject/k0sctl ;;
k0s_sort_version) pkg=github.com/k0sproject/version ;;
*) pkg='' ;;
esac

if [ -n "$pkg" ]; then
  version_from_go_mod "$pkg" <"$from"/go.mod
  exit 0
fi

exec make --no-print-directory -r -s -f - <<EOF
include $from/Makefile.variables

.PHONY: print
print:
ifeq (\$(origin $var),file)
	\$(info \$($var))
else
	\$(error Makefile variable $var doesn't exist)
endif
EOF
