
.PHONY: build
build:
	$(MAKE) -C ../../ build tmpdir=/tmp/k0s-build

.PHONY: create-cluster
create-cluster: build
	docker build -t k0s-footloose .
	envsubst < footloose.yaml.tpl > footloose.yaml
	footloose create

.PHONY: delete-cluster
delete-cluster:
	footloose delete
	docker volume prune -f
