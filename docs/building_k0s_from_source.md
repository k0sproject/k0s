# Building k0s


`k0s` can be built in 3 different ways:

Fetch official binaries (except `kine` and `konnectivity-server`, which are built from source) - (requires that Go is installed on the build system):
```
make EMBEDDED_BINS_BUILDMODE=fetch
```

Build Kubernetes components from source as static binaries (requires docker):
```
make EMBEDDED_BINS_BUILDMODE=docker
```

Build k0s without any embedded binaries (requires that Kubernetes
binaries are pre-installed on the runtime system):
```
make EMBEDDED_BINS_BUILDMODE=none
```

Builds can be done in parallel:
```
make -j$(nproc)
```

## Smoke test

To run a smoke test after build:
```
make check-basic
```
