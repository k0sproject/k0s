#!/bin/bash

set -e
CSPLIT_BINARY="csplit"

# on MacOS, we need to use the homebrew coreutils (gnu-utils) version of csplit, named gcsplit by default
if [[ "$OSTYPE" == "darwin"* ]]; then
    CSPLIT_BINARY="gcsplit"
fi

PATH=$PATH:$GOPATH/bin

DIR="static/manifests/calico"

mkdir -p $DIR

curl https://docs.projectcalico.org/manifests/calico.yaml -O

$CSPLIT_BINARY --digits=2 --quiet --prefix=$DIR/ calico.yaml "/---/" "{*}"

for f in $DIR/*
do
    # skip directories
    if [ -d $f ]; then
        continue
    fi

    filename=$(yq eval '.metadata.name' $f)
    kind=$(yq eval '.kind' $f)

    if [[ $filename == "null" || $kind == "null" ]]; then
        rm $f
        continue
    fi
    echo "Processing $kind $filename $f"
    mkdir -p $DIR/$kind
    mv $f $DIR/$kind/$filename.yaml
done


# cleanup
rm calico.yaml
