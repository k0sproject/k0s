
GO_SRCS := $(shell find -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either 'docker' or 'fetch'
EMBEDDED_BINS_BUILDMODE=docker

.PHONY: all
all: build

pkg/assets/zz_generated_bindata.go: .bins.stamp
	go-bindata -o pkg/assets/zz_generated_bindata.go \
		-pkg assets \
		-prefix embedded-bins/staging/linux/ \
		embedded-bins/staging/linux/bin \

mke: pkg/assets/zz_generated_bindata.go $(GO_SRCS)
	CGO_ENABLED=0 go build -o mke main.go

.PHONY: build
build: mke

.PHONY: bins
bins: .bins.stamp

.bins.stamp:
	$(MAKE) -C embedded-bins buildmode=$(EMBEDDED_BINS_BUILDMODE)
	touch $@

.PHONY: clean
clean:
	rm -f pkg/assets/zz_generated_bindata.go mke .bins.stamp
	$(MAKE) -C embedded-bins clean

