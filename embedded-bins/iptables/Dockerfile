ARG BUILDIMAGE
FROM $BUILDIMAGE AS build

RUN apk add build-base git file curl gpg gpg-agent gnupg-dirmngr \
	linux-headers pkgconf bison flex libmnl-dev libmnl-static libnftnl-dev

ARG VERSION
RUN set -eu \
  && for server in hkps://keyserver.ubuntu.com hkps://pgp.surf.nl hkp://pgp.rediris.es; do \
    if gpg --keyserver "$server" --keyserver-options timeout=10 --recv-keys D55D978A8A1420E4; then \
      break; \
    fi; \
  done \
  && for file in iptables-$VERSION.tar.bz2 iptables-$VERSION.tar.bz2.sig; do \
    for mirror in https://www.netfilter.org/projects/iptables/files https://web.archive.org/web/9999id_/https://www.netfilter.org/projects/iptables/files; do \
      if curl -Lfo "$file" --retry 3 --connect-timeout 10 --max-time 120 -- "$mirror/$file"; then \
        break; \
      fi; \
    done; \
  done \
  && gpg --verify iptables-$VERSION.tar.bz2.sig \
  && tar xf iptables-$VERSION.tar.bz2 \
  && echo Done!

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
