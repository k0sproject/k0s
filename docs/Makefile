include ../embedded-bins/Makefile.variables
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
	cd .. && KUBERNETES_VERSION=$(kubernetes_version) mkdocs build --strict

.PHONY: .require-mkdocs
.require-mkdocs:
	@which mkdocs >/dev/null 2>/dev/null || { \
	  echo 'mkdocs required, use pip install --disable-pip-version-check -r $(CURDIR)/requirements.txt' >&2; \
	  exit 1; \
	}

.PHONY: .k0s-download-url .k0s.exe-download-url
ifeq ($(TARGET_VERSION),latest)
.k0s-download-url .k0s.exe-download-url: release_metadata_path = repos/k0sproject/k0s/releases/latest
else
.k0s-download-url .k0s.exe-download-url: release_metadata_path = repos/k0sproject/k0s/releases/tags/$(TARGET_VERSION)
endif
.k0s-download-url .k0s.exe-download-url:
	$(eval binary_suffix = $(patsubst .k0s%,%,$(@:-download-url=)))
	gh api \
	  --jq '. as {name: $$ver, assets: $$a} | $$a[] | select(.name == "k0s-"+$$ver+"-amd64$(binary_suffix)") | .browser_download_url' \
	  -- '$(release_metadata_path)'

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
	  && curl --proto '=https' --tlsv1.2 --retry 5 --retry-all-errors -Lfo '$@.tmp' --etag-compare '$@.etag' --etag-save '$@.etag.tmp' -- "$$downloadUrl"
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

.docker-image.serve-dev.stamp: Dockerfile.serve-dev requirements_pip.txt requirements.txt Makefile.variables ../embedded-bins/Makefile.variables
	docker build --progress=plain \
	  --build-arg PYTHON_IMAGE_VERSION=$(python_version)-alpine$(alpine_version) \
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

.PHONY: update-pip-requirements
update-pip-requirements: .docker-image.serve-dev.stamp
	docker run --rm --entrypoint sh k0sdocs.docker-image.serve-dev -c \
	'pip install --disable-pip-version-check pip-tools > /dev/null \
	  && echo pip | pip-compile --allow-unsafe --generate-hashes --output-file - - | grep -E -v "^ *#" \
	  && echo wheel | pip-compile --allow-unsafe --generate-hashes --output-file - - | grep -E -v "^ *#"' \
	  > requirements_pip.txt.tmp
	mv -- requirements_pip.txt.tmp requirements_pip.txt
