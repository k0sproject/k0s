#!/bin/sh

# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# QEMU wrapper that gives a lab VM a second NIC on a point-to-point UDP link
# between local ports K0S_LAB_PORT and K0S_LAB_PEER. Lima's user-v2 network
# isn't usable here: QEMU's stream netdev aborts with
# "net_fill_rstate: Assertion `size == 0' failed" when the Windows guest
# reconfigures its NIC (e.g. when the HNS overlay network gets created).
#
#   QEMU_SYSTEM_X86_64=$PWD/qemu-lab.sh K0S_LAB_QEMU=qemu-system-x86_64 K0S_LAB_PORT=... K0S_LAB_PEER=... K0S_LAB_MAC=... limactl start ...
#
# Without K0S_LAB_PORT, QEMU runs unchanged.

set -eu

qemu=${K0S_LAB_QEMU:-qemu-system-x86_64}

[ -n "${K0S_LAB_PORT-}" ] || exec "$qemu" "$@"

exec "$qemu" "$@" \
  -netdev "socket,id=lab0,udp=127.0.0.1:$K0S_LAB_PEER,localaddr=127.0.0.1:$K0S_LAB_PORT" \
  -device "virtio-net-pci,netdev=lab0,mac=$K0S_LAB_MAC"
