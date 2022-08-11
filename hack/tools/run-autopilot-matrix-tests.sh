#!/bin/bash

# Usage:
#   TARGET_ARCH=arm64 run-autopilot-matrix-tests.sh check-ap-ha3x3,check-ap-single v1.24.2+k0s.0,v1.24.3+k0s.0
# or just to run the tests against the latest release
#   TARGET_ARCH=arm64 run-autopilot-matrix-tests.sh check-ap-ha3x3,check-ap-single
set +x

TESTS=${1:-check-ap-ha3x3}
VERSIONS="$2"
ARCH=${TARGET_ARCH:-amd64}

go install github.com/k0sproject/version/cmd/k0s_sort@v0.2.2
GOBIN="$(go env GOPATH)/bin"

if [[ -z "$VERSIONS" ]]; then
  RELEASE=$(gh release list -L 100 -R k0sproject/k0s | grep "+k0s." | grep -v Draft | cut -f 1 | $GOBIN/k0s_sort | tail -1)
  VERSIONS=$RELEASE
fi

while IFS=',' read -ra VERSION; do
  for ver in "${VERSION[@]}"; do
    curl -L -o k0s-${ver} https://github.com/k0sproject/k0s/releases/download/${ver}/k0s-${ver}-${ARCH}
    chmod +x k0s-${ver}

    while IFS=',' read -ra TESTARR; do
      for test in "${TESTARR[@]}"; do
        make -C inttest ${test} K0S_UPDATE_FROM_BIN=../k0s-${ver}
      done
    done <<< "$TESTS"
  done
done <<< "$VERSIONS"
