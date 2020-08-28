#!/usr/bin/env bash

set -euo pipefail

. ./shared.bash

trap _cleanup EXIT

_setup_cluster() {
  logline "start server"
  ./bin/footloose ssh --config $footlooseconfig root@node0 "nohup mke server >/dev/null 2>&1 &"
  logline "wait a bit ..."
  while true; do
     >/dev/null 2>&1 ./bin/footloose ssh -c $footlooseconfig root@node0 "ps | grep kube-apiserver" && break
    sleep 1
  done
  logline "create a token"
  token=$(./bin/footloose ssh --config $footlooseconfig root@node0 "mke token create --role=worker")
  ip=$(./bin/footloose ssh --config $footlooseconfig root@node0 "hostname -i")
  logline "join worker"
  ./bin/footloose ssh --config $footlooseconfig root@node1 "mke worker --server https://${ip}:6443 ${token}"
  logline "wait a bit ..."
  while true; do
    ./bin/footloose ssh -c $footlooseconfig root@node1 "ps | grep coredns" && break
    sleep 1
  done
  ./bin/footloose ssh --config $footlooseconfig root@node0 "cat /var/lib/mke/pki/admin.conf" > kubeconfig
}

_setup
title "sonobuoy[quick]: 1 controller, 1 worker"
_setup_cluster
export KUBECONFIG=./kubeconfig
(
  sleep 10
  exec ./bin/sonobuoy logs -f
)& 2>&1 | sed -le "s#^#sonobuoy:logs: #;"
logs_pid=$!
logline "run sonobuoy:"
set +e
  ./bin/sonobuoy run --mode=quick --wait=4 >/dev/null 2>&1
  result=$?
  kill $logs_pid
  wait $logs_pid
set -e
if [ "${result}" = "0" ]; then
  title "SUCCESS!!!"
  exit 0
else
  title "FAILURE!!!"
  exit $result
fi

