#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2026 k0s authors
# SPDX-License-Identifier: Apache-2.0

# Runs the CNCF AI Conformance test suite against a fresh k0s GPU cluster on
# Azure. See README.md.

set -euo pipefail

print_usage() {
  cat <<EOF
Usage: $0 --suite-sha SHA [OPTIONS]

Provisions a two-node k0s cluster on Azure (one controller, one NVIDIA T4
worker), installs the NVIDIA GPU Operator and Kueue, runs the AI Conformance
test suite and collects the artifacts needed for a CNCF submission
(junit.xml, e2e.log, results.json). The Azure resource group is deleted on
every exit path unless --keep-cluster is given.

OPTIONS:
    --k0s-version VERSION  k0s version to install, e.g. v1.36.4+k0s.0
                           (default: the current stable release)
    --suite-sha SHA        kubernetes-sigs/ai-conformance commit to run (required)
    --out DIR              Artifact root; a timestamped subdirectory is created
                           (default: ./artifacts)
    --location LOCATION    Azure location with NCASv3_T4 quota (default: southindia)
    --run-id ID            Suffix for the resource group name k0s-aiconf-ID
                           (default: \$GITHUB_RUN_ID or the current epoch)
    --keep-cluster         Do not delete the resource group; keep kubeconfig
                           and SSH key in the artifact directory
    -h, --help             Show this message

ENVIRONMENT:
    GPU_OPERATOR_VERSION   NVIDIA GPU Operator chart version (default: v26.7.0)
    KUEUE_VERSION          Kueue release (default: v0.18.2)
    GOTESTSUM_VERSION      gotestsum version (default: v1.13.0)
    CONTROLLER_SIZE        Azure VM size for the controller (default: Standard_D2s_v3)
    WORKER_SIZE            Azure VM size for the GPU worker (default: Standard_NC4as_T4_v3)
    VM_IMAGE               Azure image URN (default: Canonical:ubuntu-24_04-lts:server:latest)
    GPU_READY_TIMEOUT      Seconds to wait for nvidia.com/gpu on the node (default: 1200)

Requires: az (logged in), k0sctl, kubectl, helm, go, jq, git, ssh, ssh-keygen, ssh-keyscan.

EXIT STATUS:
    0  every test passed or was skipped; submission files are in OUT/<ts>/submission/
    1  a test failed or a stage timed out; submission files, if any, are in
       OUT/<ts>/failed/ so they cannot be submitted by accident
EOF
}

K0S_VERSION=''
SUITE_SHA=''
OUT_ROOT=./artifacts
LOCATION=southindia
RUN_ID="${GITHUB_RUN_ID:-$(date +%s)}"
KEEP_CLUSTER=n

while [ $# -gt 0 ]; do
  case "$1" in
  --k0s-version) K0S_VERSION=$2; shift 2 ;;
  --suite-sha) SUITE_SHA=$2; shift 2 ;;
  --out) OUT_ROOT=$2; shift 2 ;;
  --location) LOCATION=$2; shift 2 ;;
  --run-id) RUN_ID=$2; shift 2 ;;
  --keep-cluster) KEEP_CLUSTER=y; shift ;;
  -h | --help) print_usage; exit 0 ;;
  *) echo "Unknown argument: $1" >&2; print_usage >&2; exit 2 ;;
  esac
done

[ -n "$SUITE_SHA" ] || { echo '--suite-sha is required' >&2; exit 2; }

[ -n "$K0S_VERSION" ] || K0S_VERSION=$(curl -sSfL https://docs.k0sproject.io/stable.txt)
[ -n "$K0S_VERSION" ] || { echo "failed to resolve the stable k0s version" >&2; exit 2; }

GPU_OPERATOR_VERSION="${GPU_OPERATOR_VERSION:-v26.7.0}"
KUEUE_VERSION="${KUEUE_VERSION:-v0.18.2}"
GOTESTSUM_VERSION="${GOTESTSUM_VERSION:-v1.13.0}"
CONTROLLER_SIZE="${CONTROLLER_SIZE:-Standard_D2s_v3}"
WORKER_SIZE="${WORKER_SIZE:-Standard_NC4as_T4_v3}"
VM_IMAGE="${VM_IMAGE:-Canonical:ubuntu-24_04-lts:server:latest}"
GPU_READY_TIMEOUT="${GPU_READY_TIMEOUT:-1200}"

HERE=$(cd "$(dirname "$0")" && pwd)
RG="k0s-aiconf-$RUN_ID"
VNET=k0s-aiconf-vnet
SUBNET=k0s-aiconf-subnet
NSG=k0s-aiconf-nsg
ROUTE_TABLE=k0s-aiconf-routes
CONTROLLER_VM=k0s-controller-0
WORKER_VM=k0s-gpu-0
CONTROLLER_PRIVATE_IP=10.0.1.4
WORKER_PRIVATE_IP=10.0.1.5
ADMIN_USER=ubuntu
GANG_NAMESPACE=ai-conformance-gang-scheduling
GANG_QUEUE=e2e-lq

TIMESTAMP=$(date -u +%Y-%m-%dT%H-%MZ)
OUT="$OUT_ROOT/$TIMESTAMP"
SUBMISSION="$OUT/submission"
DEBUG="$OUT/debug"
WORK=$(mktemp -d)
SSH_KEY="$WORK/ssh/id_ed25519"
export KUBECONFIG="$OUT/kubeconfig"

CONTROLLER_IP=''
WORKER_IP=''
SUITE_RC=''

log() { printf '%s %s\n' "$(date -u +%H:%M:%SZ)" "$*" >&2; }

stage() {
  local name=$1 start rc=0
  shift
  start=$(date +%s)
  log "==> $name"
  "$@" || rc=$?
  printf '%s,%s,%s\n' "$name" "$(($(date +%s) - start))" "$rc" >>"$DEBUG/timings.csv"
  [ $rc -eq 0 ] || log "!!! $name failed (rc=$rc)"
  return $rc
}

wait_for() {
  local timeout=$1 deadline
  shift
  deadline=$(($(date +%s) + timeout))
  until "$@"; do
    [ "$(date +%s)" -lt "$deadline" ] || { log "timed out after ${timeout}s waiting for: $*"; return 1; }
    sleep 10
  done
}

ssh_cmd() {
  local host=$1
  shift
  ssh -i "$SSH_KEY" -o UserKnownHostsFile="$WORK/known_hosts" -o StrictHostKeyChecking=yes \
    -o ConnectTimeout=10 -o BatchMode=yes "$ADMIN_USER@$host" "$@"
}

check_tools() {
  local missing=''
  for tool in az k0sctl kubectl helm go jq git curl ssh ssh-keygen ssh-keyscan; do
    command -v "$tool" >/dev/null 2>&1 || missing="$missing $tool"
  done
  [ -z "$missing" ] || { echo "Missing tools:$missing" >&2; exit 2; }
  az account show --query id -o tsv >/dev/null || { echo 'az is not logged in' >&2; exit 2; }
}

collect_cluster_state() {
  [ -f "$KUBECONFIG" ] || return 0
  local dir="$DEBUG/cluster-state"
  mkdir -p "$dir"
  local k="kubectl --request-timeout=30s"
  $k get nodes -o yaml >"$dir/nodes.yaml" 2>"$dir/nodes.err" || true
  $k describe node "$WORKER_VM" >"$dir/gpu-node-describe.txt" 2>&1 || true
  $k get all -A >"$dir/get-all.txt" 2>&1 || true
  $k -n gpu-operator get pods -o wide >"$dir/gpu-operator-pods.txt" 2>&1 || true
  $k get clusterqueue,localqueue,resourceflavor -A >"$dir/kueue-objects.txt" 2>&1 || true
  $k get events -A --sort-by=.lastTimestamp >"$dir/events.txt" 2>&1 || true
}

print_summary() {
  local test rc_word
  echo
  echo "k0s $K0S_VERSION · suite ${SUITE_SHA:0:7} · $WORKER_SIZE $LOCATION"
  if [ -f "$SUBMISSION/e2e.log" ]; then
    for test in TestSecureAcceleratorAccess TestGangScheduling TestAcceleratorClusterAutoscaling; do
      grep -E "^--- (PASS|FAIL|SKIP): $test " "$SUBMISSION/e2e.log" |
        awk '{printf "%-36s %-5s %s\n", $3, $2, $4}' | tr -d ':()'
    done
  else
    echo "suite did not run"
  fi
  if [ "$KEEP_CLUSTER" = y ]; then
    rc_word="RG $RG kept (kubeconfig: $KUBECONFIG)"
  else
    rc_word="RG $RG delete issued"
  fi
  echo "artifacts: $OUT/  ·  $rc_word"
}

on_exit() {
  local rc=$1
  trap - EXIT
  set +e
  log "cleanup (rc=$rc)"
  collect_cluster_state
  for ip in $CONTROLLER_IP $WORKER_IP; do
    ssh-keygen -R "$ip" >/dev/null 2>&1
  done
  if [ "$KEEP_CLUSTER" = y ]; then
    mkdir -p "$OUT/ssh"
    cp "$SSH_KEY" "$SSH_KEY.pub" "$OUT/ssh/" 2>/dev/null
    cp "$WORK/known_hosts" "$OUT/ssh/" 2>/dev/null
  else
    az group delete -n "$RG" --yes --no-wait -o none 2>/dev/null || log "az group delete $RG failed; delete it manually"
    rm -f "$KUBECONFIG"
  fi
  # Failed runs must not be submitted by accident.
  if [ "$rc" -ne 0 ] && [ -n "$(ls -A "$SUBMISSION" 2>/dev/null)" ]; then
    mv "$SUBMISSION" "$OUT/failed"
  fi
  rmdir "$SUBMISSION" 2>/dev/null
  rm -rf "$WORK"
  print_summary
  exit "$rc"
}

provision_network() {
  az group create -n "$RG" -l "$LOCATION" -o none
  az network vnet create -g "$RG" -n "$VNET" --address-prefix 10.0.0.0/16 \
    --subnet-name "$SUBNET" --subnet-prefix 10.0.1.0/24 -o none
  az network nsg create -g "$RG" -n "$NSG" -o none
  az network nsg rule create -g "$RG" --nsg-name "$NSG" -n allow-ssh --priority 100 \
    --destination-port-ranges 22 --access Allow --protocol Tcp -o none
  az network nsg rule create -g "$RG" --nsg-name "$NSG" -n allow-k8s-api --priority 110 \
    --destination-port-ranges 6443 --access Allow --protocol Tcp -o none
}

create_vm() {
  local name=$1 size=$2 private_ip=$3
  if az vm show -g "$RG" -n "$name" -o none 2>/dev/null; then
    log "$name already exists"
    return 0
  fi
  az vm create -g "$RG" -n "$name" --image "$VM_IMAGE" --size "$size" \
    --admin-username "$ADMIN_USER" --ssh-key-values "$SSH_KEY.pub" \
    --vnet-name "$VNET" --subnet "$SUBNET" --nsg "$NSG" \
    --public-ip-sku Standard --private-ip-address "$private_ip" \
    --no-wait -o none 2> >(grep -v '^WARNING' >&2)
}

provision_vms() {
  mkdir -p "$(dirname "$SSH_KEY")"
  [ -f "$SSH_KEY" ] || ssh-keygen -q -t ed25519 -N '' -C "k0s-aiconf-$RUN_ID" -f "$SSH_KEY"
  create_vm "$CONTROLLER_VM" "$CONTROLLER_SIZE" "$CONTROLLER_PRIVATE_IP"
  create_vm "$WORKER_VM" "$WORKER_SIZE" "$WORKER_PRIVATE_IP"
  az vm wait -g "$RG" -n "$CONTROLLER_VM" --created
  az vm wait -g "$RG" -n "$WORKER_VM" --created
  CONTROLLER_IP=$(az vm show -g "$RG" -n "$CONTROLLER_VM" -d --query publicIps -o tsv)
  WORKER_IP=$(az vm show -g "$RG" -n "$WORKER_VM" -d --query publicIps -o tsv)
  log "controller $CONTROLLER_IP ($CONTROLLER_PRIVATE_IP), worker $WORKER_IP ($WORKER_PRIVATE_IP)"
}

# Azure needs IP forwarding and a route per pod CIDR for pod traffic to flow.
provision_routes() {
  local vm nic_id
  for vm in "$CONTROLLER_VM" "$WORKER_VM"; do
    nic_id=$(az vm show -g "$RG" -n "$vm" --query 'networkProfile.networkInterfaces[0].id' -o tsv)
    az network nic update --ids "$nic_id" --ip-forwarding true -o none
  done
  az network route-table create -g "$RG" -n "$ROUTE_TABLE" -l "$LOCATION" -o none
  set_pod_route "$CONTROLLER_VM" 10.244.0.0/24 "$CONTROLLER_PRIVATE_IP"
  set_pod_route "$WORKER_VM" 10.244.1.0/24 "$WORKER_PRIVATE_IP"
  az network vnet subnet update -g "$RG" --vnet-name "$VNET" -n "$SUBNET" --route-table "$ROUTE_TABLE" -o none
}

set_pod_route() {
  local node=$1 cidr=$2 next_hop=$3
  az network route-table route create -g "$RG" --route-table-name "$ROUTE_TABLE" \
    -n "pods-$node" --address-prefix "$cidr" \
    --next-hop-type VirtualAppliance --next-hop-ip-address "$next_hop" -o none 2>/dev/null ||
    az network route-table route update -g "$RG" --route-table-name "$ROUTE_TABLE" \
      -n "pods-$node" --address-prefix "$cidr" --next-hop-ip-address "$next_hop" -o none
}

ssh_reachable() { ssh_cmd "$1" true 2>/dev/null; }

wait_for_ssh() {
  local ip
  # Azure recycles public IPs; k0sctl reads ~/.ssh/known_hosts.
  for ip in "$CONTROLLER_IP" "$WORKER_IP"; do
    ssh-keygen -R "$ip" >/dev/null 2>&1 || true
    wait_for 300 sh -c "ssh-keyscan -T 10 '$ip' 2>/dev/null | grep -q ."
    ssh-keyscan -T 10 "$ip" 2>/dev/null | grep -v '^#' | tee -a "$WORK/known_hosts" >>"$HOME/.ssh/known_hosts"
    wait_for 120 ssh_reachable "$ip"
  done
}

# The NVIDIA driver is built against the running kernel.
install_kernel_headers() {
  ssh_cmd "$WORKER_IP" 'sudo DEBIAN_FRONTEND=noninteractive apt-get update -qq &&
    sudo DEBIAN_FRONTEND=noninteractive apt-get install -y -qq "linux-headers-$(uname -r)" >/dev/null'
}

k0sctl_apply() {
  cat >"$WORK/k0sctl.yaml" <<EOF
apiVersion: k0sctl.k0sproject.io/v1beta1
kind: Cluster
metadata:
  name: $RG
spec:
  hosts:
    - ssh:
        address: $CONTROLLER_IP
        user: $ADMIN_USER
        keyPath: $SSH_KEY
      role: controller+worker
    - ssh:
        address: $WORKER_IP
        user: $ADMIN_USER
        keyPath: $SSH_KEY
      role: worker
  k0s:
    version: $K0S_VERSION
    config:
      spec:
        telemetry:
          enabled: false
EOF
  cp "$WORK/k0sctl.yaml" "$DEBUG/k0sctl.yaml"
  k0sctl apply --config "$WORK/k0sctl.yaml" --kubeconfig-out "$KUBECONFIG" 2>&1 | tee "$DEBUG/k0sctl.log" |
    grep -E 'level=(warn|error|fatal)' || true
  [ -s "$KUBECONFIG" ] || { log 'k0sctl did not write a kubeconfig'; return 1; }
  kubectl get nodes -o wide
}

verify_pod_cidrs() {
  local node cidr next_hop
  while read -r node cidr; do
    case "$node" in
    "$CONTROLLER_VM") next_hop=$CONTROLLER_PRIVATE_IP ;;
    "$WORKER_VM") next_hop=$WORKER_PRIVATE_IP ;;
    *) log "unexpected node $node"; return 1 ;;
    esac
    set_pod_route "$node" "$cidr" "$next_hop"
  done < <(kubectl get nodes -o jsonpath='{range .items[*]}{.metadata.name} {.spec.podCIDR}{"\n"}{end}')
}

gpu_capacity_is_one() {
  [ "$(kubectl get node "$WORKER_VM" -o jsonpath='{.status.capacity.nvidia\.com/gpu}' 2>/dev/null)" = 1 ]
}

install_gpu_operator() {
  mkdir -p "$DEBUG/helm-values"
  cp "$HERE/gpu-operator-values.yaml" "$DEBUG/helm-values/"
  helm repo add nvidia https://helm.ngc.nvidia.com/nvidia --force-update >/dev/null
  helm upgrade --install gpu-operator nvidia/gpu-operator -n gpu-operator --create-namespace \
    --version "$GPU_OPERATOR_VERSION" -f "$HERE/gpu-operator-values.yaml" -o json | jq -r '"gpu-operator " + .info.status'
  wait_for "$GPU_READY_TIMEOUT" gpu_capacity_is_one
  kubectl -n gpu-operator get pods -o wide
}

kueue_active() {
  kubectl get clusterqueue e2e-cq -o jsonpath='{.status.conditions[?(@.type=="Active")].status}' 2>/dev/null | grep -q True
}

install_kueue() {
  kubectl apply --server-side -f "https://github.com/kubernetes-sigs/kueue/releases/download/$KUEUE_VERSION/manifests.yaml" >/dev/null
  kubectl -n kueue-system rollout status deployment kueue-controller-manager --timeout=5m
  wait_for 120 kubectl apply -f "$HERE/kueue-objects.yaml"
  wait_for 120 kueue_active
}

run_suite() {
  git clone -q https://github.com/kubernetes-sigs/ai-conformance "$WORK/ai-conformance"
  git -C "$WORK/ai-conformance" checkout -q "$SUITE_SHA"
  SUITE_SHA=$(git -C "$WORK/ai-conformance" rev-parse HEAD)
  # -autoscaler-node-pool-label is left unset: k0s ships no cluster autoscaler.
  (
    cd "$WORK/ai-conformance"
    go run "gotest.tools/gotestsum@$GOTESTSUM_VERSION" \
      --junitfile "$SUBMISSION/junit.xml" --jsonfile "$SUBMISSION/results.json" --format standard-verbose -- \
      ./test -v -timeout 30m \
      -kubeconfig="$KUBECONFIG" \
      -accelerator-type=nvidia \
      -allocation-mode=auto \
      -gang-scheduler-namespace="$GANG_NAMESPACE" \
      -gang-job-labels="kueue.x-k8s.io/queue-name=$GANG_QUEUE" \
      2>&1 | tee "$SUBMISSION/e2e.log" | { grep -E '^(=== RUN|--- (PASS|FAIL|SKIP)|PASS|FAIL|ok|DONE)' || true; }
    exit "${PIPESTATUS[0]}"
  ) || SUITE_RC=$?
  SUITE_RC=${SUITE_RC:-0}
  return "$SUITE_RC"
}

write_metadata() {
  local driver
  driver=$(kubectl -n gpu-operator exec ds/nvidia-driver-daemonset -- \
    nvidia-smi --query-gpu=driver_version,name --format=csv,noheader 2>/dev/null || echo 'unknown,unknown')
  jq -n \
    --arg date "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --arg k0s "$K0S_VERSION" \
    --arg k8s "$(kubectl version -o json | jq -r .serverVersion.gitVersion)" \
    --arg containerd "$(kubectl get node "$WORKER_VM" -o jsonpath='{.status.nodeInfo.containerRuntimeVersion}')" \
    --arg kernel "$(kubectl get node "$WORKER_VM" -o jsonpath='{.status.nodeInfo.kernelVersion}')" \
    --arg suite "$SUITE_SHA" \
    --arg gpu_operator "$GPU_OPERATOR_VERSION" \
    --arg kueue "$KUEUE_VERSION" \
    --arg driver "${driver%%,*}" \
    --arg gpu "$(echo "${driver#*,}" | xargs)" \
    --arg location "$LOCATION" --arg rg "$RG" \
    --arg controller "$CONTROLLER_SIZE" --arg worker "$WORKER_SIZE" --arg image "$VM_IMAGE" \
    --slurpfile timings <(jq -R -n '[inputs | split(",") | {key: .[0], value: (.[1] | tonumber)}] | from_entries' "$DEBUG/timings.csv") \
    '{
      date: $date, k0s_version: $k0s, kubernetes_version: $k8s, containerd: $containerd,
      suite: {repo: "https://github.com/kubernetes-sigs/ai-conformance", sha: $suite},
      gpu_operator_chart: $gpu_operator, kueue: $kueue, nvidia_driver: $driver, gpu: $gpu,
      allocation_mode: "device-plugin",
      azure: {location: $location, resource_group: $rg, controller_size: $controller,
              worker_size: $worker, image: $image, kernel: $kernel},
      timings_s: $timings[0]
    }' >"$OUT/run-metadata.json"
  cat "$OUT/run-metadata.json"
}

check_tools
mkdir -p "$SUBMISSION" "$DEBUG" "$HOME/.ssh"
: >"$DEBUG/timings.csv"
trap 'on_exit $?' EXIT
log "run $RUN_ID: k0s $K0S_VERSION, suite $SUITE_SHA, artifacts in $OUT"

stage provision-network provision_network
stage provision-vms provision_vms
stage provision-routes provision_routes
stage wait-for-ssh wait_for_ssh
install_kernel_headers &
HEADERS_PID=$!
stage k0sctl-apply k0sctl_apply
stage verify-pod-cidrs verify_pod_cidrs
stage kernel-headers wait "$HEADERS_PID"
stage gpu-operator install_gpu_operator
stage kueue install_kueue
stage test-suite run_suite
stage metadata write_metadata
