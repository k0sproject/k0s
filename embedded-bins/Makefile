
staging-pkgs = $(patsubst %/Dockerfile,staging/.%,$(wildcard */Dockerfile))

.PHONY: all
all: $(staging-pkgs) $(patsubst staging/.%,.container.%,$(staging-pkgs))

staging/linux/bin:
	mkdir -p $@

staging/.%: .container.% | staging/linux/bin
	docker export $$(cat $<) | tar -C staging/linux -xv bin/
	touch $@

.container.%: .docker-image.%.stamp
	docker create mkebuild$(basename $<) > $@.tmp
	mv $@.tmp $@

.docker-image.%.stamp: %/Dockerfile
	docker build -t mkebuild$(basename $@) -f $< .
	touch $@

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
	rm -rf staging
