#!/bin/bash
docker-compose up -d 

RET=1
until [ ${RET} -eq 0 ]; do
    token=`docker-compose exec master mke token create`
    RET=$?
    sleep 10
done
# docker-compose start worker
docker-compose exec worker nohup mke worker $token >/tmp/mke-worker.log 2>&1 &