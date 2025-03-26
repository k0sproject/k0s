ARG BUILDIMAGE
FROM $BUILDIMAGE

RUN set -ex; \
  if [ ! -z "$(which apt)" ]; then \
     apt update && apt install -y make gcc; \
  elif [ ! -z "$(which apk)" ]; then \
     apk add --no-cache make gcc musl-dev binutils-gold; \
  else \
     echo "unsupported package manager"; \
     exit 1; \
  fi


ENV \
  HOME="/run/k0s-build" \
  PATH="/run/k0s-build/go/bin:$PATH" \
  GOBIN="/run/k0s-build/go/bin" \
  GOCACHE="/run/k0s-build/go/build" \
  GOMODCACHE="/run/k0s-build/go/mod"
