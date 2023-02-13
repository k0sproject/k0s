#!/usr/bin/env sh

set -eu

export KUBECONFIG="$1/pki/admin.conf"

[ -f "$KUBECONFIG" ] || {
  echo "kubeconfig not present: $KUBECONFIG" 1>&2
  exit 1
}

bundleDir="$(mktemp -d)"
trap 'rm -rf -- "$bundleDir"' INT EXIT
kubectl supportbundle \
  --debug \
  --interactive=false \
  --output="$bundleDir/support-bundle.tar.gz" \
  /etc/troubleshoot/k0s-inttest.yaml 1>&2
cat -- "$bundleDir/support-bundle.tar.gz"
