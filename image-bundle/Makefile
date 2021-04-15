.docker-bundler.stamp: Dockerfile image.list
	docker build -t k0sbuild$(basename $@) .
	touch $@

image-bundle.tar: .docker-bundler.stamp
	docker run -t -i --rm -v $(shell pwd):/hostdir -v /var/lib/containerd --privileged k0sbuild.docker-bundler