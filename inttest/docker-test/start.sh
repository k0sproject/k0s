#!/bin/bash
make -C ../../
cp ../../mke ./mke
docker build . -t mke 
docker run -d --rm --privileged -p 6443:6443 -v /var/lib/mke mke mke server --enable-worker

sleep 2 
container_id=`docker ps  --filter "ancestor=mke" -q`
docker cp $container_id:/var/lib/mke/pki/admin.conf kubeconfig
export KUBECONFIG=kubeconfig