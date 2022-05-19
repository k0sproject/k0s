#!/usr/bin/env sh

set -eu

goVersion="$(./vars.mk go_version)"
golangciLintVersion="$(./vars.mk FROM=hack/tools golangci-lint_version)"
pythonVersion="$(./vars.mk FROM=docs python_version)"

cat <<EOF >>"$GITHUB_ENV"
GO_VERSION=$goVersion
GOLANGCI_LINT_VERSION=$golangciLintVersion
PYTHON_VERSION=$pythonVersion
EOF

# shellcheck disable=SC1090
. "$GITHUB_ENV"

echo ::group::OS Environment
env | sort
echo ::endgroup::
