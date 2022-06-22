include Makefile.variables

ifeq ($(OS),Windows_NT)
detected_OS := windows
else
detected_OS := $(shell uname | tr [:upper:] [:lower:])
endif

ifeq ($(detected_OS),darwin)
sedopt := -i "" -e
else
sedopt := -i -e
endif

TARGET_VERSION ?= latest
# mkdocs.yml refuses to live in the docs-directory, this is why "cd .." is needed
# gives: "Error: The 'docs_dir' should not be the parent directory of the config file."
# even if docs_dir is "."
docs: .require-mkdocs cli
	cd .. && mkdocs build --strict

.PHONY: .require-mkdocs
.require-mkdocs:
	@which mkdocs >/dev/null 2>/dev/null || { \
	  echo 'mkdocs required, use pip install mkdocs' >&2; \
	  exit 1; \
	}

.PHONY: .k0s-download-url .k0s.exe-download-url
ifeq ($(TARGET_VERSION),latest)
.k0s-download-url .k0s.exe-download-url: release_metadata_url = $(k0s_releases_url)/latest
else
.k0s-download-url .k0s.exe-download-url: release_metadata_url = $(k0s_releases_url)/tags/$(TARGET_VERSION)
endif
.k0s-download-url .k0s.exe-download-url:
	$(eval binary_suffix = $(patsubst .k0s%,%,$(@:-download-url=)))
	curl -sS -- '$(release_metadata_url)' \
	  | jq -r '. as {name: $$ver, assets: $$a} | $$a[] | select(.name == "k0s-"+$$ver+"-amd64$(binary_suffix)") | .browser_download_url'

.PHONY: k0s k0s.exe
ifeq ($(TARGET_VERSION),local)
cli: k0s_binary_path := $(abspath ..)
else
cli: k0s_binary_path := $(abspath .)
k0s k0s.exe::
	[ ! -f '$@.etag' ] || { if [ ! -f '$@' ] || [ '$@.etag' -ot '$@' ]; then rm -- '$@.etag'; fi }
	touch -- '$@.etag'
	@downloadUrl="$$($(MAKE) --no-print-directory -s '.$@-download-url')" \
	  && echo "Download URL for $@ $(TARGET_VERSION): $$downloadUrl" \
	  && curl -Lo '$@.tmp' --etag-compare '$@.etag' --etag-save '$@.etag.tmp' -- "$$downloadUrl"
	if [ -f '$@.tmp' ]; then \
	  mv -- '$@.tmp' '$@' && touch -r '$@' -- '$@.etag.tmp'; \
	fi
	mv -- '$@.etag.tmp' '$@.etag'

k0s::
	chmod +x -- '$@'
endif

.PHONY: cli
ifeq ($(detected_OS),windows)
cli: k0s.exe
cli: k0s_binary = k0s.exe
else
cli: k0s
cli: k0s_binary = k0s
endif
cli:
	rm -rf cli
	mkdir cli
	cd .. && '$(k0s_binary_path)/$(k0s_binary)' docs markdown
	rm cli/k0s_kubectl_*
	sed $(sedopt) '/\[k0s kubectl /d' cli/k0s_kubectl.md
	ln -s k0s.md cli/README.md

.docker-image.serve-dev.stamp: Dockerfile.serve-dev requirements.txt
	docker build \
	  --build-arg PYTHON_IMAGE_VERSION=$(python_version)-alpine3.16 \
	  -t 'k0sdocs$(basename $@)' -f '$<' .
	touch -- '$@'

clean-k0s:
	rm -f k0s k0s.exe k0s.etag k0s.exe.etag

.PHONY: clean
clean: clean-k0s
	rm -rf cli
	rm -rf ../site
	for i in .docker-image.*.stamp; do \
	  if [ -f "$$i" ]; then \
	    docker rmi -f "k0sdocs$$(basename "$$i" .stamp)" && rm -- "$$i";\
	  fi; \
	done
