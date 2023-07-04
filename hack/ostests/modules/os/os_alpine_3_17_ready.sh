#!/usr/bin/env sh

for backoff in $(seq 1 10); do
  if [ -f /var/lib/cloud/.bootstrap-complete ]; then
    read -r userDataExit </var/log/user-data.exit
    [ "$userDataExit" != 0 ] || exit 0
    cat </var/log/user-data.log 1>&2
    if ! [ "$userDataExit" -gt 0 ] 2>/dev/null; then
      exit 1
    fi
    exit "$userDataExit"
  fi

  sleep "$backoff"
done

echo Timed out while detecting readiness! 1>&2
exit 1
