#!/bin/bash

set -eEuo pipefail

AMD64_DOCKER_HOST=${AMD64_DOCKER_HOST:-"unix:///var/run/docker.sock"}
ARM_DOCKER_HOST=${ARM_DOCKER_HOST:-"tcp://raspberrypi.local:2375"}

if [ ! -x ${HOME}/.docker/cli-plugins/docker-buildx ]; then
    mkdir -p ${HOME}/.docker/cli-plugins/
    wget https://github.com/docker/buildx/releases/download/v0.7.0/buildx-v0.7.0.linux-amd64 -O ${HOME}/.docker/cli-plugins/docker-buildx
    chmod +x ${HOME}/.docker/cli-plugins/docker-buildx
fi

docker buildx version

BUILDER="multiarch-builder"
CONFIG=${PWD}/multiarch/buildkitd.toml
docker buildx create --name ${BUILDER} --buildkitd-flags '--allow-insecure-entitlement security.insecure --allow-insecure-entitlement network.host' --config=${CONFIG} --driver=docker-container --driver-opt image=moby/buildkit:latest,network=host --platform=linux/amd64 --use ${AMD64_DOCKER_HOST}
docker buildx create --name ${BUILDER} --buildkitd-flags '--allow-insecure-entitlement security.insecure --allow-insecure-entitlement network.host' --config=${CONFIG} --driver=docker-container --driver-opt image=moby/buildkit:latest,network=host --platform=linux/arm64,linux/arm/v7,linux/arm/v6 --append ${ARM_DOCKER_HOST}
stopbuilders() {
    set +x
    echo stopping builders
    docker buildx stop ${BUILDER}
    docker buildx rm ${BUILDER}
}
trap stopbuilders INT TERM EXIT

docker buildx inspect --bootstrap --builder=${BUILDER}

PLATFORM="${PLATFORM:-"linux/amd64,linux/arm/v7,linux/arm64"}"

docker --log-level=debug \
    buildx build ${PWD} \
    --builder=${BUILDER} \
    --allow security.insecure \
    --platform=${PLATFORM} \
    $@