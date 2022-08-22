#!/bin/bash

# Founds the last k0s releases of the given versions and generates json MATRIX_OUTPUT for github actions.
# Usage:
#  ./gen-matrix.sh 1.24.2 1.24.3
# Output: ["v1.24.2+k0s.0","v1.24.3+k0s.0"]

go install github.com/k0sproject/version/cmd/k0s_sort@v0.2.2
GOBIN="$(go env GOPATH)/bin"
MATRIX_OUTPUT="["
COMMA=""
for i in "$@"; do \
  RELEASE=$(gh release list -L 100 -R k0sproject/k0s | grep "+k0s." | grep -v Draft | cut -f 1 | $GOBIN/k0s_sort | grep $i | tail -1)
  MATRIX_OUTPUT+="$COMMA\"$RELEASE\""
  COMMA=","
done

MATRIX_OUTPUT+="]"

echo $MATRIX_OUTPUT
