#!/bin/bash

if [ -z "${TOOL_DATADIR}" ]; then
    echo "Error: TOOL_DATADIR environment variable required"
    exit 1
fi

export IMAGE=${IMAGE:-tool}

docker run \
    --rm \
    -e AWS_ACCESS_KEY_ID \
    -e AWS_SESSION_TOKEN \
    -e AWS_SECRET_ACCESS_KEY \
     -v "${TOOL_DATADIR}":/tool/data \
    "${IMAGE}" \
    "$@" 

