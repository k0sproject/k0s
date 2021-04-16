.docker-bundler.stamp: Dockerfile image.list
	docker build -t k0sbuild$(basename $@) .
	touch $@

bundle.tar: .docker-bundler.stamp
	docker run --rm -v $(shell pwd):/hostdir -v /var/lib/containerd --privileged k0sbuild.docker-bundler


clean:
	rm -f bundle.tar
	rm -f .docker-bundler.stamp