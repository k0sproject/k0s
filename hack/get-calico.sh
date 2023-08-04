#!/bin/sh

set -e
CALICO_VERSION="$1"
CSPLIT_BINARY="csplit"

if [ -z "$CALICO_VERSION" ]; then
  echo "usage: $0 <VERSION>"
  exit 1
fi

# on MacOS, we need to use the homebrew coreutils (gnu-utils) version of csplit, named gcsplit by default
case "$(uname -s)" in
  darwin*) CSPLIT_BINARY="gcsplit" ;;
esac

if ! command -v "$CSPLIT_BINARY" > /dev/null; then
  echo "$CSPLIT_BINARY not found" >&2
  exit 2
fi

PATH=$PATH:$GOPATH/bin

DIR="static/manifests/calico"

mkdir -p $DIR

curl --silent -L "https://raw.githubusercontent.com/projectcalico/calico/v$CALICO_VERSION/manifests/calico.yaml" \
  | $CSPLIT_BINARY --digits=2 --quiet --prefix=$DIR/ -- - "/---/" "{*}"

for f in "$DIR"/*; do
  # skip directories
  if [ -d "$f" ]; then
    continue
  fi

  filename=$(yq eval '.metadata.name' "$f")
  kind=$(yq eval '.kind' "$f")

  if [ "$filename" = "null" ] || [ "$kind" = "null" ]; then
    rm "$f"
    continue
  fi
  echo "Processing $kind $filename $f"
  mkdir -p "$DIR/$kind"
  mv "$f" "$DIR/$kind/$filename.yaml"
done
