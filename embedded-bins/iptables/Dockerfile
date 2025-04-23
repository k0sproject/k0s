ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN if [ ! -z "$(which apt)" ]; then \
       apt update && apt install -y build-essential curl \
        pkg-config \
        libmnl-dev libnftnl-dev; \
    elif [ ! -z "$(which apk)" ]; then \
       apk add build-base curl pkgconf \
         linux-headers \
         libmnl-dev libmnl-static \
         libnftnl-dev; \
    else \
       echo "unsupported package manager"; \
       exit 1; \
    fi

ARG VERSION
RUN curl --proto '=https' --tlsv1.2 -L https://www.netfilter.org/projects/iptables/files/iptables-$VERSION.tar.xz \
	| tar -C / -Jx

ARG TARGET_OS
# -D__UAPI_DEF_ETHHDR is required to build on musl.
# If this CFLAG isn't defined, itpables will define it in include/xtables.h
# and will have a conflict with musl, which defines it in
# /usr/include/netinet/if_ether.h
RUN if [ ! -z "$(which apk)" ]; then \
      cd /iptables-$VERSION && \
      CFLAGS="-Os -D__UAPI_DEF_ETHHDR=0" ./configure --sysconfdir=/etc --enable-static --disable-shared --without-kernel --disable-devel; \
    else \
      cd /iptables-$VERSION && \
      CFLAGS="-Os" ./configure --sysconfdir=/etc --enable-static --disable-shared --without-kernel --disable-devel; \
    fi

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
