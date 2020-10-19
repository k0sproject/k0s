#!/bin/bash
docker-compose up -d master 

RET=1
until [ ${RET} -eq 0 ]; do
    sleep 10
    token=`docker-compose exec master mke token create`
    RET=$?
done

TOKEN=$token docker-compose up -d worker