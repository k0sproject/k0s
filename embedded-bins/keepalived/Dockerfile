ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN if [ ! -z "$(which apt)" ]; then \
       apt update && apt install -y build-essential curl \
        openssl libssl-dev \
        libnl-3-dev libnl-3-200 libnl-genl-3-dev; \
    elif [ ! -z "$(which apk)" ]; then \
       apk add build-base curl \
        linux-headers \
        openssl-dev openssl-libs-static \
        libnl3-dev libnl3-static; \
    else \
       echo "unsupported package manager"; \
       exit 1; \
    fi

ARG VERSION
RUN curl --proto '=https' --tlsv1.2 -L https://www.keepalived.org/software/keepalived-$VERSION.tar.gz \
	| tar -C / -zx

RUN cd /keepalived-$VERSION \
	&& CFLAGS='-static -s' LDFLAGS=-static ./configure  --disable-dynamic-linking \
	&& make -j$(nproc)

FROM scratch
ARG VERSION
COPY --from=build /keepalived-$VERSION/bin/keepalived \
	/bin/keepalived
