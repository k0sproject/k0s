setup_file() {
	export name=$(basename ${BATS_TEST_FILENAME%.bats})
	export netname=mke-test-$name
	export footlooseconfig=footloose-$name.yaml

	docker network inspect $netname 2>/dev/null \
		|| docker network create $netname

	CLUSTER_NAME=$name \
		LINUX_IMAGE=quay.io/footloose/ubuntu18.04 \
		NETWORK_NAME=$netname \
		MKE_BINARY=${MKE_BINARY:-$(readlink -f ../mke)} \
		envsubst < ${footlooseconfig}.tpl > $footlooseconfig

	footloose create --config $footlooseconfig
	export node0=$(printf "$(footloose --config $footlooseconfig show -o json \
		| jq --raw-output '.machines[0].spec.name')\n" 0)

	footloose ssh --config $footlooseconfig root@$node0 "addgroup --system mke"
}

teardown_file() {
	footloose delete --config $footlooseconfig
	docker network rm $netname
}

