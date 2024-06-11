#!/usr/bin/env sh
# SPDX-License-Identifier: Apache-2.0
# SPDX-FileCopyrightText: Copyright 2024 k0s authors

# Try to use only shell builtins here, PATH might not be available.

set -eu

pipe="$1"
ignoreTerm="${2-}"

# Some signal handling stuff. Required to make this portable
# across shells, some of them won't retry syscalls on EINTR.
signalsReceived=0
handle_signal() {
  signalsReceived=$((signalsReceived + 1))
  [ -n "$ignoreTerm" ] || exit 0
}
trap handle_signal TERM

# Send ping, ignoring EINTR
while :; do
  seen=$signalsReceived
  { echo ping >"$pipe" && break; } || {
    ret=$?
    [ $signalsReceived -ne $seen ] || exit $ret
  }
done

# Receive pong, ignoring EINTR
while :; do
  seen=$signalsReceived
  { read -r _ <"$pipe" && break; } || {
    ret=$?
    [ $signalsReceived -ne $seen ] || exit $ret
  }
done
