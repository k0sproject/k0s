#!/usr/bin/env bash
set -x

terraform output controller_pem > aws_private.pem
terraform output -json > out.json

# prepare private key
chmod 0600 aws_private.pem

# github actions' terraform print debug information on the first line
# this command removes it
sed -i '1d' out.json
sed -i '1d' aws_private.pem
