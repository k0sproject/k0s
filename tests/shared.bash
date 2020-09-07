title() {
  echo "==> TEST: ${1}"
}

logline() {
  echo "    ${1}"
}

_setup() {
  export name=$(basename ${0%.bash})
  export footlooseconfig=footloose-$name.yaml

  echo "==> SETUP"
  logline "creating footloose config ..."
  CLUSTER_NAME=$name \
    LINUX_IMAGE=quay.io/footloose/ubuntu18.04 \
    MKE_BINARY=${MKE_BINARY:-$(readlink -f ../mke)} \
    envsubst < ${footlooseconfig}.tpl > $footlooseconfig

  logline "starting to create footloose nodes ..."
  >/dev/null 2>&1 ./bin/footloose create --config $footlooseconfig

  logline "create mke groups on nodes ..."
  >/dev/null 2>&1 ./bin/footloose ssh --config $footlooseconfig root@node0 "addgroup --system mke"
  >/dev/null 2>&1 ./bin/footloose ssh --config $footlooseconfig root@node1 "addgroup --system mke"
  >/dev/null 2>&1 ./bin/footloose ssh --config $footlooseconfig root@node2 "addgroup --system mke"
}

_cleanup() {
  echo ""
  echo "==> CLEANUP"
  set +e

  _collect_logs
  logline "cleaning up footloose cluster"
  >/dev/null 2>&1 ./bin/footloose delete --config $footlooseconfig
  >/dev/null 2>&1 rm -f $footlooseconfig

  logline "pruning docker volumes"
  >/dev/null 2>&1 docker volume prune -f
}

_collect_logs() {
  logline "Collecting logs"
  nodes=("node0" "node1" "node2")
  for node in "${nodes[@]}";
  do
    >$node.log ./bin/footloose ssh --config $footlooseconfig root@$node "cat /tmp/mke-*.log"  
  done
}