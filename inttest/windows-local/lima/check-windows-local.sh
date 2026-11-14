#!/bin/sh

# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Creates a Linux controller VM and a Windows worker VM with Lima, runs the
# inttest/windows smoke test against them, and deletes the VMs again. This is
# the local equivalent of the win-wsl.yaml workflow.
#
#   check-windows-local.sh build-base
#   check-windows-local.sh <path/to/k0s> <path/to/k0s.exe>
#   check-windows-local.sh clean
#   check-windows-local.sh clean-all
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
export QEMU_SYSTEM_X86_64="$labDir/qemu-lab.sh"
export QEMU_SYSTEM_AARCH64="$labDir/qemu-lab.sh"

# The controller runs natively (Lima's default arch), the Windows worker is
# always x86_64 (emulated on arm64 hosts).
case "$(uname -m)" in
arm64 | aarch64) controllerQemu=qemu-system-aarch64 controllerFirmware=edk2-aarch64-code.fd ;;
*) controllerQemu=qemu-system-x86_64 controllerFirmware=edk2-x86_64-code.fd ;;
esac

# Lima looks for the UEFI firmware relative to the QEMU binary it is told to
# use, which is the wrapper here. Link the firmware of the real QEMU next to it.
linkFirmware() {
  for qemuBin in qemu-system-x86_64 "$controllerQemu"; do
    case "$qemuBin" in
    qemu-system-aarch64) fwName=edk2-aarch64-code.fd ;;
    *) fwName=edk2-x86_64-code.fd ;;
    esac
    qemu=$(command -v "$qemuBin") || {
      echo "$qemuBin not found in PATH" >&2
      return 1
    }
    qemuDir=$(cd -- "$(dirname -- "$(realpath -- "$qemu")")" && pwd)
    found=0
    for fw in "$qemuDir/../share/qemu/$fwName" "$qemuDir/../share/$fwName"; do
      if [ -r "$fw" ]; then
        mkdir -p -- "$labDir/share"
        ln -sf -- "$fw" "$labDir/share/$fwName"
        found=1
        break
      fi
    done
    [ $found -eq 1 ] || {
      echo "UEFI firmware $fwName not found next to $qemu" >&2
      return 1
    }
  done
}

instanceExists() {
  limactl list -q 2>/dev/null | grep -qxF -- "$1"
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

# The test talks to the API via Lima's port forwarding to 127.0.0.1:6443.
apiReachable() {
  curl -sk -o /dev/null --max-time 2 https://127.0.0.1:6443/
}

# Starts a VM with its end of the lab link (see qemu-lab.sh). The MACs are
# matched by controller.yaml and provision-worker.ps1.
startLabVM() {
  K0S_LAB_QEMU=$5 K0S_LAB_PORT=$2 K0S_LAB_PEER=$3 K0S_LAB_MAC=$4 limactl start --tty=false --timeout=30m "$1" </dev/null
}

winScript() {
  limactl shell "$1" powershell -NoProfile -ExecutionPolicy Bypass -File "C:/k0s-lab/$2"
}

winCopy() {
  vm=$1
  shift
  for file; do
    limactl copy "$file" "$vm:C:/k0s-lab/$(basename -- "$file")"
  done
}

case "${1-}" in
clean)
  clean
  exit 0
  ;;
clean-all)
  clean
  deleteInstances "$base"
  exit 0
  ;;
build-base)
  deleteInstances "$base"
  trap 'status=$?; trap - EXIT; [ $status -eq 0 ] || deleteInstances "$base" || :; exit $status' EXIT
  trap 'exit 130' INT TERM

  # The first start installs Windows from the ISO, which takes a while.
  linkFirmware
  echo "Starting Windows base VM"
  limactl start --tty=false --timeout=60m --name="$base" "$labDir/windows-2025.yaml" </dev/null
  limactl shell "$base" powershell -NoProfile -Command New-Item -ItemType Directory -Force -Path C:/k0s-lab >/dev/null
  winCopy "$base" "$labDir/install-containers.ps1"

  # Fails while the feature is pending and while the VM reboots.
  echo "Installing the Containers feature"
  retry 120 10 winScript "$base" install-containers.ps1

  # Turn off Windows Update and pre-pull the images the clones need. The image
  # names and the containerd version are taken from the k0s sources.
  echo "Preparing the base VM"
  constants="$repoDir/pkg/constant/constant.go"
  imageRef() {
    name=$(sed -nE "s/^[[:space:]]*$1[[:space:]]*=[[:space:]]*\"(.*)\"/\1/p" "$constants")
    version=$(sed -nE "s/^[[:space:]]*${1}Version[[:space:]]*=[[:space:]]*\"(.*)\"/\1/p" "$constants")
    [ -n "$name" ] && [ -n "$version" ] || {
      echo "Image $1 not found in $constants" >&2
      return 1
    }
    echo "$name:$version"
  }
  mkdir -p -- "$workDir"
  {
    for image in KubePauseWindowsContainerImage KubeProxyWindowsImage CalicoCNIWindowsImage \
      CalicoNodeWindowsImage TraefikImage KonnectivityImage; do
      imageRef "$image"
    done
    # Used by inttest/windows.
    echo "mcr.microsoft.com/windows/servercore/iis:latest"
  } >"$workDir/images.txt"
  containerdVersion=$(sed -nE 's/^containerd_version_and_rev[[:space:]]*=[[:space:]]*([^@[:space:]]+)@.*/\1/p' "$repoDir/embedded-bins/Makefile.variables")
  winCopy "$base" "$labDir/prepare-base.ps1" "$workDir/images.txt"
  limactl shell "$base" powershell -NoProfile -ExecutionPolicy Bypass -File C:/k0s-lab/prepare-base.ps1 -ContainerdVersion "$containerdVersion"

  limactl stop "$base"
  exit 0
  ;;
esac

[ $# -eq 2 ] || {
  echo "Usage: $0 <path/to/k0s> <path/to/k0s.exe> | build-base | clean | clean-all" >&2
  exit 1
}

k0s=$(realpath -- "$1")
k0sExe=$(realpath -- "$2")

instanceExists "$base" || {
  echo "Lima instance $base doesn't exist, build it with: $0 build-base" >&2
  exit 1
}
for vm in "$controller" "$worker"; do
  if instanceExists "$vm"; then
    echo "Lima instance $vm already exists, remove it with: $0 clean" >&2
    exit 1
  fi
done
if apiReachable; then
  echo "Port 6443 is already in use on 127.0.0.1" >&2
  exit 1
fi

linkFirmware
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
limactl create --tty=false --name="$controller" "$labDir/controller.yaml" </dev/null
startLabVM "$controller" 10001 10002 52:54:00:77:00:01 "$controllerQemu"

echo "Installing k0s controller"
limactl copy "$k0s" "$controller:/tmp/k0s"
limactl shell "$controller" sudo sh -euc '
  install -m755 /tmp/k0s /usr/local/bin/k0s
  k0s install controller --enable-worker --no-taints --disable-components=metrics-server --kubelet-extra-args=--node-ip=10.77.0.1
  systemctl start k0scontroller.service
'
retry 120 1 limactl shell "$controller" sudo k0s kubectl get --raw=/readyz >/dev/null 2>&1
retry 30 1 apiReachable
limactl shell "$controller" sudo k0s kubeconfig admin |
  sed -E 's#(server: https://)[^:/]+(:6443)#\1127.0.0.1\2#' >"$workDir/kubeconfig"
# The status socket that "token create" needs may show up after the API is ready.
createWorkerToken() {
  limactl shell "$controller" sudo k0s token create --role=worker >"$workDir/k0s-worker-token"
}
retry 120 1 createWorkerToken

echo "Starting Windows worker VM from $base"
limactl clone --tty=false "$base" "$worker" </dev/null
startLabVM "$worker" 10002 10001 52:54:00:77:00:02 qemu-system-x86_64

echo "Installing k0s worker"
winCopy "$worker" "$k0sExe" "$workDir/k0s-worker-token" "$labDir/provision-worker.ps1"
winScript "$worker" provision-worker.ps1

echo "Running the Windows smoke test"
cd -- "$repoDir"
KUBECONFIG="$workDir/kubeconfig" go test -v -count=1 -timeout="${K0S_WINDOWS_TEST_TIMEOUT:-3h}" ./inttest/windows
