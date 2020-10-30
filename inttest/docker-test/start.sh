#!/bin/bash
make -C ../../
docker-compose down --rmi all -v 
docker-compose up -d master 

echo "Waiting for master to start"
RET=1
until [ ${RET} -eq 0 ]; do
    sleep 2
    token=`docker-compose exec master ./mke token create`
    RET=$?
done

TOKEN=$token docker-compose up -d worker

docker-compose exec master cat /var/lib/mke/pki/admin.conf > kubeconfig
export KUBECONFIG=kubeconfig


#this is a dirty solution as it assumes that only two nodes are being provisioned. 
echo "Wait for nodes to become ready"
while [[ $(kubectl get nodes  -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True True" ]]; do sleep 1; done

docker-compose exec master /bin/sh ./script.sh
coredns=`kubectl get pods --all-namespaces -o=name |  sed "s/^.\{4\}//" | grep coredns`
kubectl delete pods -n kube-system $coredns

kubectl get nodes 
