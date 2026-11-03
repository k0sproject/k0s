#!/bin/sh

# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Creates a Linux controller VM and a Windows worker VM with Lima, runs the
# inttest/windows smoke test against them, and deletes the VMs again. This is
# the local equivalent of the win-wsl.yaml workflow.
#
#   lab.sh build-base
#   lab.sh test <path/to/k0s> <path/to/k0s.exe>
#   lab.sh clean
#   lab.sh clean-all
#
# build-base installs Windows and its Containers feature into a stopped base
# instance, which each test run clones. The Makefile runs it when needed.
#
# Honors K0S_KEEP_AFTER_TESTS like the other integration tests (never, always,
# failure). "clean" removes what a test run left behind, "clean-all" removes
# the base instance as well.

set -eu

labDir=$(cd -- "$(dirname -- "$0")" && pwd)
repoDir=$(cd -- "$labDir/../../.." && pwd)
workDir="$labDir/.work"

controller='k0s-lab-controller'
worker='k0s-lab-worker'
base='k0s-lab-worker-base'

# The lab link: a point-to-point UDP connection between the VMs (see
# qemu-wrapper.sh), carrying the k0s API, kubelet and Calico traffic.
linkCIDR='10.77.0.0/24'
linkPrefixLength=24
controllerIP='10.77.0.1'
controllerMAC='52:54:00:77:00:01'
controllerPort=10001
workerIP='10.77.0.2'
workerMAC='52:54:00:77:00:02'
workerPort=10002
workerNodeName='windowsworker'
podCIDR='10.244.0.0/16' # k0s default
apiPort=6443

export QEMU_SYSTEM_X86_64="$labDir/qemu-wrapper.sh"

die() {
  echo "$*" >&2
  exit 1
}

instanceExists() {
  names=$(limactl list -q) || die "Failed to list Lima instances"
  printf '%s\n' "$names" | grep -qxF -- "$1"
}

deleteInstances() {
  command -v limactl >/dev/null || return 0
  for vm; do
    if instanceExists "$vm"; then
      limactl delete -f "$vm"
    fi
  done
}

clean() {
  deleteInstances "$controller" "$worker"
  rm -rf -- "$workDir"
}

# retry <tries> <delay> <command...>
retry() {
  tries=$1 delay=$2
  shift 2
  i=1
  until "$@"; do
    [ $i -lt "$tries" ] || return 1
    i=$((i + 1))
    sleep "$delay"
  done
}

# The test talks to the API via Lima's port forwarding to 127.0.0.1.
apiReachable() {
  curl -sk -o /dev/null --max-time 2 "https://127.0.0.1:$apiPort/"
}

controllerReady() {
  limactl shell "$controller" sudo k0s kubectl get --raw=/readyz >/dev/null 2>&1
}

udpPortInUse() {
  command -v ss >/dev/null || return 1
  ss -Hlun "sport = :$1" | grep -q .
}

preflight() {
  for cmd in limactl qemu-system-x86_64 curl; do
    command -v "$cmd" >/dev/null || die "$cmd is required"
  done
  [ -w /dev/kvm ] || die "/dev/kvm is not writable, KVM is required"
}

# Starts a VM with its end of the lab link (see qemu-wrapper.sh).
# startLabVM <vm> <local port> <peer port> <MAC>
startLabVM() {
  K0S_LAB_PORT=$2 K0S_LAB_PEER=$3 K0S_LAB_MAC=$4 limactl start --tty=false --timeout=30m "$1"
}

winShell() {
  vm=$1
  shift
  limactl shell "$vm" powershell -NoProfile -ExecutionPolicy Bypass "$@"
}

winScript() {
  vm=$1 script=$2
  shift 2
  winShell "$vm" -File "C:/k0s-lab/$script" "$@"
}

winCopy() {
  vm=$1
  shift
  for file; do
    limactl copy "$file" "$vm:C:/k0s-lab/$(basename -- "$file")"
  done
}

# Runs install-containers.ps1 until the feature is installed. The script exits
# with 3010 when it has scheduled a reboot, which SSH reports modulo 256 (194),
# and the SSH connection fails (255) while the VM is rebooting. Everything else
# is a real error.
installContainers() {
  tries=120
  i=0
  while [ $i -lt $tries ]; do
    i=$((i + 1))
    status=0
    winScript "$base" install-containers.ps1 || status=$?
    case $status in
    0) return 0 ;;
    194 | 255) sleep 10 ;;
    *) die "Installing the Containers feature failed with exit code $status" ;;
    esac
  done
  die "Timed out waiting for the Containers feature to be installed"
}

buildBase() {
  deleteInstances "$base"
  trap 'status=$?; trap - EXIT; [ $status -eq 0 ] || deleteInstances "$base" || :; exit $status' EXIT
  trap 'exit 130' INT TERM

  # The first start installs Windows from the ISO, which takes a while.
  echo "Starting Windows base VM"
  limactl start --tty=false --timeout=60m --name="$base" "$labDir/worker-base.yaml"
  # The arguments must stay unquoted: the remote shell is cmd.exe.
  winShell "$base" -Command New-Item -ItemType Directory -Force -Path C:/k0s-lab >/dev/null
  winCopy "$base" "$labDir/install-containers.ps1"

  echo "Installing the Containers feature"
  installContainers

  limactl stop "$base"
}

runTest() {
  [ $# -eq 2 ] || die "Usage: $0 test <path/to/k0s> <path/to/k0s.exe>"
  k0s=$(realpath -- "$1")
  k0sExe=$(realpath -- "$2")

  preflight
  instanceExists "$base" || die "Lima instance $base doesn't exist, build it with: $0 build-base"
  for vm in "$controller" "$worker"; do
    if instanceExists "$vm"; then
      die "Lima instance $vm already exists, remove it with: $0 clean"
    fi
  done
  ! apiReachable || die "Port $apiPort is already in use on 127.0.0.1"
  for port in "$controllerPort" "$workerPort"; do
    ! udpPortInUse "$port" || die "UDP port $port is already in use on 127.0.0.1"
  done

  mkdir -p -- "$workDir"
  cleanup() {
    status=$?
    trap - EXIT INT TERM
    case "${K0S_KEEP_AFTER_TESTS-}" in
    always) keep=1 ;;
    failure) [ $status -eq 0 ] && keep=0 || keep=1 ;;
    *) keep=0 ;;
    esac
    if [ "$keep" = 1 ]; then
      echo "Lima instances left intact for debugging, KUBECONFIG=$workDir/kubeconfig." >&2
      echo "Needs to be manually cleaned with: $0 clean" >&2
    else
      clean || :
    fi
    exit $status
  }
  trap cleanup EXIT
  trap 'exit 130' INT TERM

  echo "Starting controller VM"
  limactl create --tty=false --name="$controller" "$labDir/controller.yaml"
  startLabVM "$controller" "$controllerPort" "$workerPort" "$controllerMAC"

  echo "Configuring controller VM"
  limactl shell "$controller" sudo sh -euc "
    cat >/etc/netplan/99-k0s-lab.yaml <<EOF
network:
  version: 2
  ethernets:
    lab0:
      match:
        macaddress: \"$controllerMAC\"
      set-name: lab0
      dhcp4: false
      addresses: [$controllerIP/$linkPrefixLength]
EOF
    chmod 600 /etc/netplan/99-k0s-lab.yaml
    netplan apply

    mkdir -p /etc/k0s
    cat >/etc/k0s/k0s.yaml <<EOF
spec:
  api:
    address: $controllerIP
  storage:
    type: kine
  network:
    provider: calico
    calico:
      ipAutodetectionMethod: cidr=$linkCIDR
    nodeLocalLoadBalancing:
      enabled: true
      type: Traefik
EOF
  "

  echo "Installing k0s controller"
  limactl copy "$k0s" "$controller:/tmp/k0s"
  limactl shell "$controller" sudo sh -euc "
    install -m755 /tmp/k0s /usr/local/bin/k0s
    k0s install controller --enable-worker --no-taints --disable-components=metrics-server --kubelet-extra-args=--node-ip=$controllerIP
    systemctl start k0scontroller.service
  "
  retry 120 1 controllerReady || {
    limactl shell "$controller" sudo k0s kubectl get --raw=/readyz?verbose || :
    die "Controller didn't become ready"
  }
  retry 30 1 apiReachable || die "API server isn't reachable on 127.0.0.1:$apiPort"

  # k0s includes 127.0.0.1 in the API server certificate's SANs, so the
  # forwarded port can be used instead of the address on the lab link.
  limactl shell "$controller" sudo k0s kubeconfig admin |
    sed -E "s#(server: https://)[^:/]+(:$apiPort)#\\1127.0.0.1\\2#" >"$workDir/kubeconfig"
  limactl shell "$controller" sudo k0s token create --role=worker >"$workDir/k0s-worker-token"

  echo "Starting Windows worker VM from $base"
  limactl clone --tty=false "$base" "$worker"
  startLabVM "$worker" "$workerPort" "$controllerPort" "$workerMAC"

  echo "Installing k0s worker"
  winCopy "$worker" "$k0sExe" "$workDir/k0s-worker-token" "$labDir/provision-worker.ps1"
  winScript "$worker" provision-worker.ps1 \
    -LabMac "$(printf '%s' "$workerMAC" | tr ':a-f' '-A-F')" \
    -LabIP "$workerIP" \
    -LabPrefixLength "$linkPrefixLength" \
    -LabCIDR "$linkCIDR" \
    -PodCIDR "$podCIDR" \
    -NodeName "$workerNodeName"

  echo "Running the Windows smoke test"
  cd -- "$repoDir"
  KUBECONFIG="$workDir/kubeconfig" go test -v -count=1 -timeout=60m ./inttest/windows
}

case "${1-}" in
build-base)
  preflight
  buildBase
  ;;
test)
  shift
  runTest "$@"
  ;;
clean)
  clean
  ;;
clean-all)
  clean
  deleteInstances "$base"
  ;;
*)
  die "Usage: $0 build-base | test <path/to/k0s> <path/to/k0s.exe> | clean | clean-all"
  ;;
esac
