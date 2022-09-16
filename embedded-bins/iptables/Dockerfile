ARG BUILDIMAGE=alpine:3.16
FROM $BUILDIMAGE AS build

ARG VERSION
RUN apk add build-base git file curl \
	linux-headers pkgconf bison flex libmnl-dev libmnl-static libnftnl-dev

RUN curl -L https://www.netfilter.org/projects/iptables/files/iptables-$VERSION.tar.bz2 \
	| tar -C / -jx

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
CMD ["/bin/xtables-legacy-multi"]