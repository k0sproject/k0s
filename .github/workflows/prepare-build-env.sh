#!/usr/bin/env sh

set -eu

goVersion="$(./vars.sh go_version)"
golangciLintVersion="$(./vars.sh FROM=hack/tools golangci-lint_version)"
cosignVersion="$(./vars.sh FROM=hack/tools cosign_version)"
pythonVersion="$(./vars.sh FROM=docs python_version)"

cat <<EOF >>"$GITHUB_ENV"
GO_VERSION=$goVersion
GOLANGCI_LINT_VERSION=$golangciLintVersion
COSIGN_VERSION=$cosignVersion
PYTHON_VERSION=$pythonVersion
EOF

echo ::group::OS Environment
env | sort
echo ::endgroup::

echo ::group::Build Environment
cat -- "$GITHUB_ENV"
echo ::endgroup::
