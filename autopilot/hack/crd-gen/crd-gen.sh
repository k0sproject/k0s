#!/bin/sh

if [ $# -ne 3 ]; then
    echo "Usage: $0 <name> <group> <version>"
    echo "    Example: $0 autopilot autopilot.k0sproject.io v1beta2"
    exit 1
fi

export ARG_NAME=$1
export ARG_GROUP=$2
export ARG_VERSION=$3

set -eu

docker build \
    -t autopilot-client-gen \
    -f hack/crd-gen/Dockerfile.crd-gen \
    hack/crd-gen

echo "--> Running 'controller-gen' for ${ARG_GROUP}/${ARG_VERSION}"
docker run \
    --workdir /go/src/autopilot/pkg/apis/${ARG_GROUP}/${ARG_VERSION} \
    --user $(id -u) \
    --rm \
    -v $(pwd):/go/src/autopilot \
    autopilot-client-gen \
        controller-gen \
            crd \
            paths="./..." \
            output:crd:artifacts:config=/go/src/autopilot/embedded/manifests/${ARG_GROUP}/${ARG_VERSION} \
            object

# client-gen seems to always want to include `output-package` as a part of the
# output path, resulting in long repetitive package paths. This volume-mount
# indirection filters off this path noise and keeps the package paths small.
# Example: pkg/apis/autopilot.k0sproject.io/v1beta2/clientset/...

echo "--> Running 'client-gen' for ${ARG_GROUP}/${ARG_VERSION}"
docker run \
    --user $(id -u) \
    --rm \
    -v $(pwd):/go/src/autopilot \
    -v $(pwd)/pkg/apis:/tmp/github.com/k0sproject/autopilot/pkg/apis \
    autopilot-client-gen \
        client-gen \
            --go-header-file /go/src/autopilot/hack/crd-gen/boilerplate.go.txt \
            --input-base github.com/k0sproject/autopilot/pkg/apis \
            --input ${ARG_GROUP}/${ARG_VERSION} \
            --clientset-name clientset \
            --output-base /tmp \
            --output-package github.com/k0sproject/autopilot/pkg/apis/${ARG_GROUP}/${ARG_VERSION}
