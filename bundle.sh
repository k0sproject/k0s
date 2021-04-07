#!/bin/bash

set -Eeuo pipefail
set -x
OUTPUT=${OUTPUT:-"bundle"}
K0S_BINARY=${K0S_BINARY:-"./k0s"}
CTR_BIN=${CTR_BIN:-"ctr"}
CONTAINERD_RUN_SOCKET=${CONTAINERD_RUN_SOCKET:-"/run/k0s/containerd.sock"}
CTR_CMD="${CTR_BIN} --namespace bundle_builder --address ${CONTAINERD_RUN_SOCKET}"

function get_images() {
  ${K0S_BINARY} airgap list-images | xargs
}

function ensure_images() {
  for image in $(get_images); do
    ${CTR_CMD} images pull $image
  done
}

function pack_images() {
  ${CTR_CMD} images export $OUTPUT $(get_images)
}

function build_bundle() {
  ensure_images
  pack_images
}

build_bundle
