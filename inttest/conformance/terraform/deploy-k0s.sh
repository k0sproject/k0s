#!/usr/bin/env bash
set -x
K0S_BIN="${GITHUB_WORKSPACE}/k0s"
PRIVATE_KEY="${GITHUB_WORKSPACE}/inttest/conformance/terraform/aws_private.pem"
SSH_OPTS="-o StrictHostKeyChecking=no"

chmod +x ${K0S_BIN}

# prepare private key
chmod 0600 ${PRIVATE_KEY}

controller_ips=$(cat out.json| jq -r '.["controller_external_ip"].value[]' 2> /dev/null)
worker_ips=$(cat out.json| jq -r '.["worker_external_ip"].value[]' 2> /dev/null)

# remove single quotes, if exists
controller_ips=(${controller_ips[@]//\'/})
worker_ips=(${worker_ips[@]//\'/})

# Save To File
echo $controller_ips > CTRL_IPS
echo $worker_ips > WORKER_IPS

for controller in "${controller_ips[@]}"
do
  scp ${SSH_OPTS} -i ${PRIVATE_KEY} $K0S_BIN ubuntu@"${controller}":
  ssh ${SSH_OPTS} -i ${PRIVATE_KEY} ubuntu@"${controller}" "sudo scp k0s /usr/local/bin/k0s"
done

for worker in "${worker_ips[@]}"
do
  scp ${SSH_OPTS} -i ${PRIVATE_KEY} $K0S_BIN ubuntu@"${worker}":
  ssh ${SSH_OPTS} -i ${PRIVATE_KEY} ubuntu@"${worker}" "sudo scp k0s /usr/local/bin/k0s"
done