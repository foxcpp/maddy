FROM docker.io/library/golang:1.19 AS build-env

WORKDIR /maddy

COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod/ go mod download

COPY . .
RUN --mount=type=cache,target=/go/pkg/mod/ mkdir -p /pkg/data && \
    cp maddy.conf.docker /pkg/data/maddy.conf && \
    ./build.sh --builddir /tmp --destdir /pkg/ --tags docker build install

FROM gcr.io/distroless/base-debian12:latest
LABEL maintainer="fox.cpp@disroot.org"
LABEL org.opencontainers.image.source=https://github.com/foxcpp/maddy

COPY --from=build-env /pkg/data/maddy.conf /data/maddy.conf
COPY --from=build-env /pkg/usr/local/bin/maddy /

EXPOSE 25 143 993 587 465
VOLUME ["/data"]
ENTRYPOINT ["/maddy", "-config", "/data/maddy.conf"]
CMD ["run"]
