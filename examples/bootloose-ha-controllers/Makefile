
.PHONY: build
build:
	$(MAKE) -C ../../ build tmpdir=/tmp/k0s-build

.PHONY: create-cluster
create-cluster: build
	docker build --progress=plain -t k0s-bootloose .
	envsubst < bootloose.yaml.tpl > bootloose.yaml
	bootloose create

.PHONY: delete-cluster
delete-cluster:
	bootloose delete
	docker volume prune -f
