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

TARGET_VERSION ?= latest
# mkdocs.yml refuses to live in the docs-directory, this is why "cd .." is needed
# gives: "Error: The 'docs_dir' should not be the parent directory of the config file."
# even if docs_dir is "."
docs: cli
	cd .. && mkdocs build --strict

LATEST_RELEASE_META_URL=https://api.github.com/repos/k0sproject/k0s/releases/$(TARGET_VERSION)
JQ_COMMAND=jq -r '.assets[]|select(.browser_download_url|contains("amd64"))|select(.browser_download_url|contains(".exe")|not)|select(.browser_download_url|contains("bundle")|not).browser_download_url'

.PHONY: k0s_release_binary
k0s_release_binary:
	curl -L -o ./k0s `curl $(LATEST_RELEASE_META_URL) | $(JQ_COMMAND)`
	chmod +x ./k0s

.PHONY: cli
cli: k0s_release_binary
	rm -rf cli
	mkdir cli
	cd .. && ./docs/k0s docs markdown
	rm ./k0s
	rm cli/k0s_kubectl_*
	sed $(sedopt) '/\[k0s kubectl /d' cli/k0s_kubectl.md
	ln -s k0s.md cli/README.md

.PHONY: clean
clean:
	rm -rf cli
	rm -rf ../site
