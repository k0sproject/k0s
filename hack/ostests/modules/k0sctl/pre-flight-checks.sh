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

countSeenControllers() {
  kubectl -n kube-system logs "$1" \
    | grep -F '"Start serving"' \
    | sed -E 's/.+serverID="([0-9a-f]+)".+/\1/g' \
    | sort -u \
    | wc -l
}

testKonnectivityPods() {
  pods="$(kubectl -n kube-system get po -l k8s-app=konnectivity-agent --field-selector=status.phase=Running -oname)"

  seenPods=0
  for pod in $pods; do
    seenControllers="$(countSeenControllers "$pod")"
    echo Pod: "$pod", seen controllers: "$seenControllers" >&2
    [ "$seenControllers" -ne "$expectedControllers" ] || seenPods=$((seenPods + 1))
  done

  echo Seen pods with expected amount of controller connections: $seenPods >&2
  [ $seenPods -eq "$expectedWorkers" ]
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
