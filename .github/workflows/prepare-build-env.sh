#!/usr/bin/env sh

set -eu

goVersion="$(make --no-print-directory -s -f ./vars.mk go_version)"
golangciLintVersion="$(make --no-print-directory -s -f ./vars.mk FROM=hack/tools golangci-lint_version)"
pythonVersion="$(make --no-print-directory -s -f ./vars.mk FROM=docs python_version)"

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
