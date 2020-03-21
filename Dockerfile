FROM golang:1.13-alpine3.11 AS build-env

COPY . maddy/
WORKDIR maddy/

ENV LDFLAGS -static
RUN apk --no-cache add bash git gcc musl-dev

RUN mkdir /pkg/
COPY maddy.conf /pkg/data/maddy.conf
# Monkey-patch config to use environment.
RUN sed -Ei 's!\$\(hostname\) = .+!$(hostname) = {env:MADDY_HOSTNAME}!' /pkg/data/maddy.conf
RUN sed -Ei 's!\$\(primary_domain\) = .+!$(primary_domain) = {env:MADDY_DOMAIN}!' /pkg/data/maddy.conf
RUN sed -Ei 's!^tls .+!tls /data/tls_cert.pem /data/tls_key.pem!' /pkg/data/maddy.conf

RUN ./build.sh --builddir /tmp --destdir /pkg/ --configdir /data --statedir /data --runtimedir /tmp package install_pkg

FROM alpine:3.11
LABEL maintainer="fox.cpp@disroot.org"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /pkg/data/maddy.conf /data/maddy.conf
COPY --from=build-env /pkg/usr/local/bin/maddy /bin/maddy
COPY --from=build-env /pkg/usr/local/bin/maddyctl /bin/maddyctl

EXPOSE 25 143 993 587 465
VOLUME ["/data"]
ENTRYPOINT ["/bin/maddy", "-config", "/data/maddy.conf"]
