ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base curl \
	linux-headers \
	openssl-dev openssl-libs-static \
	libnl3-dev libnl3-static

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
