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
}

_cleanup() {
  set +e
    >/dev/null 2>&1 ./bin/footloose delete --config $footlooseconfig
    >/dev/null 2>&1 rm -f $footlooseconfig
    >/dev/null 2>&1 docker volume prune -f
}
