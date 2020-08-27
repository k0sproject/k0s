
.PHONY: build
build:
	$(MAKE) -C ../../ build tmpdir=/tmp/mke-build

.PHONY: cluster
cluster: build
	docker-compose up --build -d

.PHONY: teardown
teardown:
	docker-compose stop
	docker-compose rm -f
