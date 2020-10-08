#!/bin/bash

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

    filename=$(yq r $f 'metadata.name')
    kind=$(yq r $f 'kind')

    if [[ $filename == "" || $kind == "" ]]; then
        rm $f
        continue
    fi
    echo "Processing $kind $filename"
    mkdir -p $DIR/$kind
    mv $f $DIR/$kind/$filename.yaml
done

# if we need to fetch other manifests in the future, we'll want to move
# the go-bindata generation out to a separate command.
go-bindata -o static/gen_calico.go -pkg static -prefix static $DIR/...

# cleanup
rm calico.yaml
