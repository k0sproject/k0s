#!/usr/bin/env sh

set -eu

goVersion="$(./vars.sh go_version)"
k0sctlVersion="$(./vars.sh FROM=hack/tool k0sctl_version)"
golangciLintVersion="$(./vars.sh FROM=hack/tools golangci-lint_version)"
pythonVersion="$(./vars.sh FROM=docs python_version)"

cat <<EOF >>"$GITHUB_ENV"
GO_VERSION=$goVersion
GOLANGCI_LINT_VERSION=$golangciLintVersion
K0SCTL_VERSION=$k0sctlVersion
PYTHON_VERSION=$pythonVersion
EOF

echo ::group::OS Environment
env | sort
echo ::endgroup::

echo ::group::Build Environment
cat -- "$GITHUB_ENV"
echo ::endgroup::
