#!/usr/bin/env bash
export PR_NUMBER=$(echo ${GITHUB_REF} | cut -d / -f 3 )
export GITHUB_SHA_SHORT=$(git rev-parse --short ${GITHUB_SHA})
export TF_VAR_cluster_name="k0s_pr_${PR_NUMBER}_${GITHUB_SHA_SHORT}"

echo $TF_VAR_cluster_name > CLUSTER_NAME

# Upgrade jq
os=$(go env GOOS)
arch=$(go env GOARCH)

jq_url="https://github.com/stedolan/jq/releases/download/jq-1.6/jq-${os}${arch: -2}"
curl -L ${jq_url} --output /tmp/jq && sudo chmod +x /tmp/jq && sudo mv /tmp/jq /usr/bin/jq