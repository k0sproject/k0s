
.PHONY: build
build:
	$(MAKE) -C ../../ build tmpdir=/tmp/mke-build

.PHONY: create-cluster
cluster: build
	envsubst < footloose.yaml.tpl > footloose.yaml
	footloose create

.PHONY: delete-cluster
delete-cluster:
	footloose delete
	docker volume prune -f
