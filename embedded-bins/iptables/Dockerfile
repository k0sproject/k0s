ARG BUILDIMAGE=alpine:3.14
FROM $BUILDIMAGE AS build

ARG VERSION
RUN apk add build-base git file curl \
	linux-headers pkgconf libnftnl-dev bison flex

RUN curl -L https://www.netfilter.org/projects/iptables/files/iptables-$VERSION.tar.bz2 \
	| tar -C / -jx

COPY \
  use-uint-instead-of-u_int.patch \
  revert-fix-eth-alen.patch \
  /
RUN cd /iptables-$VERSION && \
  git apply /use-uint-instead-of-u_int.patch && \
  git apply /revert-fix-eth-alen.patch && \
  CFLAGS="-Os" ./configure --sysconfdir=/etc --disable-shared --disable-nftables --without-kernel --disable-devel

RUN make -j$(nproc) -C /iptables-$VERSION LDFLAGS=-all-static
RUN make -j$(nproc) -C /iptables-$VERSION install

RUN strip /usr/local/sbin/xtables-legacy-multi
RUN scanelf -Rn /usr/local && file /usr/local/sbin/*

FROM scratch
COPY --from=build /usr/local/sbin/xtables-legacy-multi \
	/bin/xtables-legacy-multi

CMD ["/bin/xtables-legacy-multi"]
