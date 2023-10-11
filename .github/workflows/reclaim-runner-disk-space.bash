#!/usr/bin/env bash

set +eux

if [ -z "$GITHUB_RUN_ID" ]; then
  echo "Cowardly refusing to destroy a machine that doesn't look like a GitHub runner." >&2
  exit 0
fi

df -h /

docker system prune --all --force &
sudo rm -rf /imagegeneration/installers &
sudo rm -rf -- "${ANDROID_SDK_ROOT-/opt/nevermind}" &

wait

df -h /
