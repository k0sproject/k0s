#!/bin/bash
# make -C ../../
# cp ../../mke ./mke

# docker network create --subnet=172.18.0.0/16 k0snet
docker build . -t mke 
docker run -dt --rm --privileged -p 6443:6443 -v /var/lib/mke --net k0snet --ip 172.18.0.22 --name master mke mke server --enable-worker

sleep 5
container_id=`docker ps -f name=master -q`
docker cp $container_id:/var/lib/mke/pki/admin.conf kubeconfig
docker exec $container_id /bin/sh fixresolv.sh
export KUBECONFIG=kubeconfig

# sleep 120

token=`docker exec $container_id mke token create`

docker run -dt --rm --privileged -v /var/lib/mke --net k0snet --ip 172.18.0.23 --name worker1 mke mke worker $token
# container_id=`docker ps -f name=master -q`
# docker exec $container_id /bin/sh fixresolv.sh
# container_id=`docker ps -f name=worker1 -q`
# docker exec $container_id /bin/sh fixresolv.sh

# coredns=`kubectl get pods --template '{{range .items}}{{.metadata.name}}{{"\n"}}{{end}}' -n kube-system  | grep coredns`
# kubectl delete pods -n kube-system $coredns

# docker run -dt --rm --privileged -v /var/lib/mke --net k0snet --ip 172.18.0.24 --name worker2 mke mke worker $token
# container_id=`docker ps -f name=worker2 -q`
# docker exec $container_id /bin/sh fixresolv.sh


while [[ $(kubectl get nodes  -o 'jsonpath={..status.conditions[?(@.type=="Ready")].status}') != "True True" ]]; do sleep 1; done

kubectl get nodes 
kubectl get pods --all-namespaces 