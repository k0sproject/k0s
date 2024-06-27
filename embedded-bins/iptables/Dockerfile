ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base curl pkgconf \
	linux-headers \
	libmnl-dev libmnl-static \
	libnftnl-dev

ARG VERSION
RUN curl --proto '=https' --tlsv1.2 -L https://www.netfilter.org/projects/iptables/files/iptables-$VERSION.tar.xz \
	| tar -C / -Jx

ARG TARGET_OS
RUN cd /iptables-$VERSION && \
  CFLAGS="-Os" ./configure --sysconfdir=/etc --enable-static --disable-shared --without-kernel --disable-devel

RUN make -j$(nproc) -C /iptables-$VERSION LDFLAGS=-all-static
RUN make -j$(nproc) -C /iptables-$VERSION install

RUN strip /usr/local/sbin/xtables-legacy-multi
RUN strip /usr/local/sbin/xtables-nft-multi
RUN scanelf -Rn /usr/local && file /usr/local/sbin/*

FROM scratch
COPY --from=build /usr/local/sbin/xtables-legacy-multi \
	/bin/xtables-legacy-multi
COPY --from=build /usr/local/sbin/xtables-nft-multi \
	/bin/xtables-nft-multi
