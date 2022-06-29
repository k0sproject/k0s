TARGET_PLATFORM ?= linux/amd64

.docker-bundler.stamp: Dockerfile image.list bundler.sh
	docker build -t k0sbuild$(basename $@) .
	touch $@

bundle.tar: .docker-bundler.stamp
	docker run --rm -v $(shell pwd):/hostdir -v /var/lib/containerd -e TARGET_PLATFORM=$(TARGET_PLATFORM) --privileged k0sbuild.docker-bundler


clean:
	rm -f bundle.tar
	rm -f .docker-bundler.stamp