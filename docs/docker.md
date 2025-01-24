# Docker

Official Docker image is available from Docker Hub.

It expects configuration file to be available at /data/maddy.conf.

If /data is a Docker volume, then default configuration will be placed there
automatically. If it is used, then MADDY_HOSTNAME, MADDY_DOMAIN environment
variables control the host name and primary domain for the server. TLS
certificate should be placed in /data/tls/fullchain.pem, private key in
/data/tls/privkey.pem

DKIM keys are generated in /data/dkim_keys directory.

## Image tags

- `latest` - A latest stable release. May contain breaking changes.
- `X.Y` - A specific feature branch, it is recommended to use these tags to
  receive bugfixes without the risk of feature-related regressions or breaking
  changes.
- `X.Y.Z` - A specific stable release

## Ports

All standard ports, as described in maddy docs.

- `25` - SMTP inbound port.
- `465`, `587` - SMTP Submission ports
- `993`, `143` - IMAP4 ports

## Volumes

`/data` - maddy state directory. Databases, queues, etc are stored here. You
might want to mount a named volume there. The main configuration file is stored
here too (`/data/maddy.conf`).

## Management utility

To run management commands, create a temporary container with the same
/data directory and put the command after the image name, like this:

```
docker run --rm -it -v maddydata:/data foxcpp/maddy:0.7 creds create foxcpp@maddy.test
docker run --rm -it -v maddydata:/data foxcpp/maddy:0.7 imap-acct create foxcpp@maddy.test
```

Use the same image version as the running server. Things may break badly
otherwise.

Note that, if you modify messages using maddy subcommands while the server is running -
you must ensure that  /tmp from the server is accessible for the management
command. One way to it is to run it using `docker exec` instead of `docker run`:
```
docker exec -it container_name_here maddy creds create foxcpp@maddy.test
```

## Build Tags

Some Maddy features (such as automatic certificate management via ACME with [a non-default libdns provider](../reference/tls-acme/#dns-providers)) require build tags to be passed to Maddy's `build.sh`, as this is run in the Dockerfile you must compile your own Docker image. Build tags can be set via the docker build argument `ADDITIONAL_BUILD_TAGS` e.g. `docker build --build-arg ADDITIONAL_BUILD_TAGS="libdns_acmedns libdns_route53" -t yourorgname/maddy:yourtagname .`.


## TL;DR

```
docker volume create maddydata
docker run \
  --name maddy \
  -e MADDY_HOSTNAME=mx.maddy.test \
  -e MADDY_DOMAIN=maddy.test \
  -v maddydata:/data \
  -p 25:25 \
  -p 143:143 \
  -p 465:465 \
  -p 587:587 \
  -p 993:993 \
  foxcpp/maddy:0.7
```

It will fail on first startup. Copy TLS certificate to /data/tls/fullchain.pem
and key to /data/tls/privkey.pem. Run the server again. Finish DNS configuration
(DKIM keys, etc) as described in [tutorials/setting-up/](../tutorials/setting-up/).
