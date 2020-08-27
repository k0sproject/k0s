setup_file() {
	export name=$(basename ${BATS_TEST_FILENAME%.bats})
	export netname=mke-test-$name
	export footlooseconfig=footloose-$name.yaml

	bin/docker network inspect $netname 2>/dev/null \
		|| bin/docker network create $netname

	CLUSTER_NAME=$name \
		LINUX_IMAGE=quay.io/footloose/ubuntu18.04 \
		NETWORK_NAME=$netname \
		MKE_BINARY=${MKE_BINARY:-$(readlink -f ../mke)} \
		envsubst < ${footlooseconfig}.tpl > $footlooseconfig
	echo "footloose config created"
	echo "starting to create footloose nodes"

	bin/footloose create --config $footlooseconfig
	export node0=$(printf "$(bin/footloose --config $footlooseconfig show -o json \
		| jq --raw-output '.machines[0].spec.name')\n" 0)

	bin/footloose ssh --config $footlooseconfig root@$node0 "addgroup --system mke"
	
}

teardown_file() {
	bin/footloose delete --config $footlooseconfig
	bin/docker network rm $netname
}

