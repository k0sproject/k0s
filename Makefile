
GO_SRCS := $(shell find -name '*.go')

# EMBEDDED_BINS_BUILDMODE can be either:
#   docker	builds the binaries in docker
#   fetch	fetch precompiled binaries from internet (except kine)
#   none	does not embed any binaries

EMBEDDED_BINS_BUILDMODE ?= fetch


.PHONY: all
all: build

ifeq ($(EMBEDDED_BINS_BUILDMODE),none)
pkg/assets/zz_generated_bindata.go:
	printf "%s\n\n%s" \
		"package assets" \
		"func Asset(name string) ([]byte, error) { return nil, nil }" \
		> $@
else

pkg/assets/zz_generated_bindata.go: .bins.stamp
	go-bindata -o $@ \
		-pkg assets \
		-prefix embedded-bins/staging/linux/ \
		embedded-bins/staging/linux/bin

endif


mke: pkg/assets/zz_generated_bindata.go $(GO_SRCS)
	CGO_ENABLED=0 go build -ldflags="-w -s" -o mke main.go

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

