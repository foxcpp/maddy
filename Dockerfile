FROM golang:1.17-alpine AS build-env

RUN set -ex ;\
    apk upgrade --no-cache --available ;\
    apk add --no-cache bash git build-base

WORKDIR /maddy
ADD go.mod go.sum ./
ENV LDFLAGS -static
RUN go mod download
ADD . ./
RUN mkdir -p /pkg/data
COPY maddy.conf /pkg/data/maddy.conf
# Monkey-patch config to use environment.
RUN sed -Ei 's!\$\(hostname\) = .+!$(hostname) = {env:MADDY_HOSTNAME}!' /pkg/data/maddy.conf
RUN sed -Ei 's!\$\(primary_domain\) = .+!$(primary_domain) = {env:MADDY_DOMAIN}!' /pkg/data/maddy.conf
RUN sed -Ei 's!^tls .+!tls file /data/tls_cert.pem /data/tls_key.pem!' /pkg/data/maddy.conf

RUN ./build.sh --builddir /tmp --destdir /pkg/ --tags docker build install

FROM alpine:3.15.0
LABEL maintainer="fox.cpp@disroot.org"
LABEL org.opencontainers.image.source=https://github.com/foxcpp/maddy

RUN set -ex ;\
    apk upgrade --no-cache --available ;\
    apk --no-cache add ca-certificates
COPY --from=build-env /pkg/data/maddy.conf /data/maddy.conf
COPY --from=build-env /pkg/usr/local/bin/maddy /pkg/usr/local/bin/maddyctl /bin/

EXPOSE 25 143 993 587 465
VOLUME ["/data"]
ENTRYPOINT ["/bin/maddy", "-config", "/data/maddy.conf"]
