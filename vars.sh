#!/usr/bin/env sh

set -eu

from=
var=

print_usage() {
  echo 'Query Makefile variables from scripts. Use like so:'
  echo
  echo "  $0 go_version"
  echo "  $0 FROM=docs python_version"
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
