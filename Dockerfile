FROM golang:1.18-alpine AS build-env

RUN set -ex && \
    apk upgrade --no-cache --available && \
    apk add --no-cache build-base

WORKDIR /maddy

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN mkdir -p /pkg/data && \
    cp maddy.conf.docker /pkg/data/maddy.conf && \
    ./build.sh --builddir /tmp --destdir /pkg/ --tags docker build install

FROM alpine:3.17.0
LABEL maintainer="fox.cpp@disroot.org"
LABEL org.opencontainers.image.source=https://github.com/foxcpp/maddy

RUN set -ex && \
    apk upgrade --no-cache --available && \
    apk --no-cache add ca-certificates
COPY --from=build-env /pkg/data/maddy.conf /data/maddy.conf
COPY --from=build-env /pkg/usr/local/bin/maddy /bin/

EXPOSE 25 143 993 587 465
VOLUME ["/data"]
ENTRYPOINT ["/bin/maddy", "-config", "/data/maddy.conf"]
CMD ["run"]
