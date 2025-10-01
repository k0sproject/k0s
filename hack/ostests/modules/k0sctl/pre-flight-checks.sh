#!/usr/bin/env sh

set -eu

print_usage() {
  echo Usage: "$0" EXPECTED_CONTROLLERS EXPECTED_WORKERS
}

findPathToK0s() {
  for k0s in /usr/local/bin/k0s /opt/bin/k0s; do
    if [ -f "$k0s" ]; then
      echo "$k0s" && return 0
    fi
  done

  which k0s
}

kubectl() {
  set -- "$k0s" kubectl "$@"
  for cmd in sudo doas; do
    if which -- "$cmd" >/dev/null 2>/dev/null; then
      set -- "$cmd" "$@"
      break
    fi
  done

  "$@"
}

testKonnectivityPods() {
  nodeIPs=$(kubectl get nodes -o jsonpath='{range .items[*]}{.status.addresses[?(@.type=="InternalIP")].address}{"\n"}{end}')

  goodNodes=0
  for ip in $nodeIPs; do
    openServerConnections=$(curl -sSf "http://$ip:8093/metrics" | {
      while read -r metric value; do
        if [ "$metric" = konnectivity_network_proxy_agent_open_server_connections ]; then
          echo "$value"
          break
        fi
      done
    })

    echo Node IP: "$ip", open konnectivity server connections: "$openServerConnections" >&2
    [ "$openServerConnections" -ne "$expectedControllers" ] || goodNodes=$((goodNodes + 1))
  done

  echo Nodes with expected number of open konnectivity server connections: $goodNodes >&2
  [ $goodNodes -eq "$expectedWorkers" ]
}

testCoreDnsDeployment() {
  kubectl -n kube-system wait --for=condition=Available --timeout=9s deploy/coredns
}

main() {
  {
    [ $# -eq 2 ] \
      && expectedControllers="$1" \
      && [ "$expectedControllers" -ge 0 ] 2>/dev/null \
      && expectedWorkers="$2" \
      && [ "$expectedWorkers" -ge 0 ] 2>/dev/null
  } || {
    print_usage >&2
    exit 1
  }

  k0s="$(findPathToK0s)"

  echo Expecting "$expectedWorkers" pods with "$expectedControllers" controller connections each ... >&2

  failedAttempts=0
  while ! testKonnectivityPods; do
    failedAttempts=$((failedAttempts + 1))
    if [ $failedAttempts -gt 30 ]; then
      echo Giving up after $failedAttempts failed attempts ... >&2
      return 1
    fi
    echo Attempt $failedAttempts failed, retrying in ten seconds ... >&2
    sleep 10
  done

  echo Waiting for CoreDNS deployment to become available ... >&2

  failedAttempts=0
  while ! testCoreDnsDeployment; do
    failedAttempts=$((failedAttempts + 1))
    if [ $failedAttempts -gt 30 ]; then
      echo Giving up after $failedAttempts failed attempts ... >&2
      return 1
    fi
    echo Attempt $failedAttempts failed, retrying in a second ... >&2
    sleep 1
  done
}

main "$@"
