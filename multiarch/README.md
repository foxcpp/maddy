# Mutliarch builds

## Requirements

An ARM64 server with docker daemon exposed (for example, a raspberry pi 4 with Raspberry Pi OS 64bits)

## Build

At repository root, launch :

```
./docker-build-multiarch.sh --tag=TAG --push
```

It will build and push multi-arch docker images as TAG.
