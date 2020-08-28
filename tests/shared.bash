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
	bin/footloose create --config $footlooseconfig
  logline "create mke groups on nodes ..."
	bin/footloose ssh --config $footlooseconfig root@node0 "addgroup --system mke"
  bin/footloose ssh --config $footlooseconfig root@node1 "addgroup --system mke"
}

_cleanup() {
  set +e
    bin/footloose delete --config $footlooseconfig
    rm -f $footlooseconfig
    docker volume prune -f
}
