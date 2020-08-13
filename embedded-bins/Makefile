
tarballs = $(patsubst %/Dockerfile,%.tar,$(wildcard */Dockerfile))

%.tar: .container.%
	docker export $$(cat $<) > $@.tmp
	mv $@.tmp $@

.container.%: .docker-image.%.stamp
	docker create mkebuild$(basename $<) > $@.tmp
	mv $@.tmp $@

.docker-image.%.stamp: %/Dockerfile
	docker build -t mkebuild$(basename $@) -f $< .
	touch $@

.PHONY: all
all: $(tarballs) $(patsubst %.tar,.container.%,$(tarballs))

.PHONY: clean
clean:
	for i in .container.*; do \
		if [ -f $$i ]; then \
			docker rm $$(cat $$i) && rm $$i; \
		fi; \
	done
	for i in .docker-image.*; do \
		if [ -f $$i ]; then \
			docker rmi mkebuild$$(basename $$i .stamp) \
				&& rm $$i;\
		fi; \
	done
	rm *.tar
