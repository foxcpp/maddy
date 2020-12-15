FROM golang:1.15.5-alpine3.12 AS build-env

COPY . maddy/
WORKDIR maddy/

ENV LDFLAGS -static
RUN apk --no-cache add bash git gcc musl-dev scdoc

RUN mkdir /pkg/
COPY maddy.conf /pkg/data/maddy.conf
# Monkey-patch config to use environment.
RUN sed -Ei 's!\$\(hostname\) = .+!$(hostname) = {env:MADDY_HOSTNAME}!' /pkg/data/maddy.conf
RUN sed -Ei 's!\$\(primary_domain\) = .+!$(primary_domain) = {env:MADDY_DOMAIN}!' /pkg/data/maddy.conf
RUN sed -Ei 's!^tls .+!tls /data/tls_cert.pem /data/tls_key.pem!' /pkg/data/maddy.conf
RUN sed -Ei 's!# state_dir /var/lib/maddy!state_dir /data!' /pkg/data/maddy.conf
RUN sed -Ei 's!# runtime_dir /run/maddy!runtime_dir /tmp!' /pkg/data/maddy.conf

RUN ./build.sh --builddir /tmp --destdir /pkg/ build install

FROM alpine:3.12
LABEL maintainer="fox.cpp@disroot.org"

RUN apk --no-cache add ca-certificates
COPY --from=build-env /pkg/data/maddy.conf /data/maddy.conf
COPY --from=build-env /pkg/usr/local/bin/maddy /bin/maddy
COPY --from=build-env /pkg/usr/local/bin/maddyctl /bin/maddyctl

EXPOSE 25 143 993 587 465
VOLUME ["/data"]
ENTRYPOINT ["/bin/maddy", "-config", "/data/maddy.conf"]
