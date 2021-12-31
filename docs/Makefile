mkdocs := $(shell which mkdocs)
ifeq ($(mkdocs),)
@echo "mkdocs required, use pip install mkdocs"
endif

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

# mkdocs.yml refuses to live in the docs-directory, this is why "cd .." is needed
# gives: "Error: The 'docs_dir' should not be the parent directory of the config file."
# even if docs_dir is "."
docs: cli
	cd .. && mkdocs build --strict

.PHONY: cli
cli:
	rm -rf cli
	mkdir cli
	$(MAKE) -C .. static/gen_manifests.go EMBEDDED_BINS_BUILDMODE=none
	$(MAKE) -C .. generate-bindata TARGET_OS=$(detected_OS)
	cd .. && go run main.go docs markdown
	rm cli/k0s_kubectl_*
	sed $(sedopt) '/\[k0s kubectl /d' cli/k0s_kubectl.md
	ln -s k0s.md cli/README.md

.PHONY: clean
clean:
	rm -rf cli
	rm -rf ../site
