#!/usr/bin/env sh

set -eu

goVersion="$(./vars.sh go_version)"
golangciLintVersion="$(./vars.sh FROM=hack/tools golangci-lint_version)"
pythonVersion="$(./vars.sh FROM=docs python_version)"

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
