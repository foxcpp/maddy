# Remote MX delivery

Module that implements message delivery to remote MTAs discovered via DNS MX
records. You probably want to use it with queue module for reliability.

If a message check marks a message as 'quarantined', remote module
will refuse to deliver it.

## Configuration directives

```
target.remote {
    hostname mx.example.org
    debug no
}
```

**Syntax**: hostname _domain_ <br>
**Default**: global directive value

Hostname to use client greeting (EHLO/HELO command). Some servers require it to
be FQDN, SPF-capable servers check whether it corresponds to the server IP
address, so it is better to set it to a domain that resolves to the server IP.

**Syntax**: limits _config block_ <br>
**Default**: no limits

See ['limits' directive for SMTP endpoint](/reference/endpoints/smtp/#rate-concurrency-limiting).
It works the same except for address domains used for
per-source/per-destination are as observed when message exits the server.

**Syntax**: local\_ip _IP address_ <br>
**Default**: empty

Choose the local IP to bind for outbound SMTP connections.

**Syntax**: force\_ipv4 _boolean_ <br>
**Default**: false

Force resolving outbound SMTP domains to IPv4 addresses. Some server providers
do not offer a way to properly set reverse PTR domains for IPv6 addresses; this
option makes maddy only connect to IPv4 addresses so that its public IPv4 address
is used to connect to that server, and thus reverse PTR checks are made against
its IPv4 address.

Warning: this may break sending outgoing mail to IPv6-only SMTP servers.

**Syntax**: connect\_timeout _duration_ <br>
**Default**: 5m

Timeout for TCP connection establishment.

RFC 5321 recommends 5 minutes for "initial greeting" that includes TCP
handshake. maddy uses two separate timers - one for "dialing" (DNS A/AAAA
lookup + TCP handshake) and another for "initial greeting". This directive
configures the former. The latter is not configurable and is hardcoded to be
5 minutes.

**Syntax**: command\_timeout _duration_ <br>
**Default**: 5m

Timeout for any SMTP command (EHLO, MAIL, RCPT, DATA, etc).

If STARTTLS is used this timeout also applies to TLS handshake.

RFC 5321 recommends 5 minutes for MAIL/RCPT and 3 minutes for
DATA.

**Syntax**: submission\_timeout _duration_ <br>
**Default**: 12m

Time to wait after the entire message is sent (after "final dot").

RFC 5321 recommends 10 minutes.

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging.

**Syntax**: requiretls\_override _boolean_ <br>
**Default**: true

Allow local security policy to be disabled using 'TLS-Required' header field in
sent messages. Note that the field has no effect if transparent forwarding is
used, message body should be processed before outbound delivery starts for it
to take effect (e.g. message should be queued using 'queue' module).

**Syntax**: relaxed\_requiretls _boolean_ <br>
**Default**: true

This option disables strict conformance with REQUIRETLS specification and
allows forwarding of messages 'tagged' with REQUIRETLS to MXes that are not
advertising REQUIRETLS support. It is meant to allow REQUIRETLS use without the
need to have support from all servers. It is based on the assumption that
server referenced by MX record is likely the final destination and therefore
there is only need to secure communication towards it and not beyond.

**Syntax**: conn\_reuse\_limit _integer_ <br>
**Default**: 10

Amount of times the same SMTP connection can be used.
Connections are never reused if the previous DATA command failed.

**Syntax**: conn\_max\_idle\_count _integer_ <br>
**Default**: 10

Max. amount of idle connections per recipient domains to keep in cache.

**Syntax**: conn\_max\_idle\_time _integer_ <br>
**Default**: 150 (2.5 min)

Amount of time the idle connection is still considered potentially usable.

## Security policies

**Syntax**: mx\_auth _config block_ <br>
**Default**: no policies

'remote' module implements a number of of schemes and protocols necessary to
ensure security of message delivery. Most of these schemes are concerned with
authentication of recipient server and TLS enforcement.

To enable mechanism, specify its name in the mx\_auth directive block:
```
mx_auth {
	dane
	mtasts
}
```
Additional configuration is possible if supported by the mechanism by
specifying additional options as a block for the corresponding mechanism.
E.g.
```
mtasts {
	cache ram
}
```

If the mx\_auth directive is not specified, no mechanisms are enabled. Note
that, however, this makes outbound SMTP vulnerable to a numberous downgrade
attacks and hence not recommended.

It is possible to share the same set of policies for multiple 'remote' module
instances by defining it at the top-level using 'mx\_auth' module and then
referencing it using standard & syntax:
```
mx_auth outbound_policy {
	dane
	mtasts {
		cache ram
	}
}

# ... somewhere else ...

deliver_to remote {
	mx_auth &outbound_policy
}

# ... somewhere else ...

deliver_to remote {
	mx_auth &outbound_policy
	tls_client { ... }
}
```

### MTA-STS

Checks MTA-STS policy of the recipient domain. Provides proper authentication
and TLS enforcement for delivery, but partially vulnerable to persistent active
attacks.

Sets MX level to "mtasts" if the used MX matches MTA-STS policy even if it is
not set to "enforce" mode.

```
mtasts {
	cache fs
	fs_dir StateDirectory/mtasts_cache
}
```

**Syntax**: cache fs|ram <br>
**Default**: fs

Storage to use for MTA-STS cache. 'fs' is to use a filesystem directory, 'ram'
to store the cache in memory.

It is recommended to use 'fs' since that will not discard the cache (and thus
cause MTA-STS security to disappear) on server restart. However, using the RAM
cache can make sense for high-load configurations with good uptime.

**Syntax**: fs\_dir _directory_ <br>
**Default**: StateDirectory/mtasts\_cache

Filesystem directory to use for policies caching if 'cache' is set to 'fs'.

### DNSSEC

Checks whether MX records are signed. Sets MX level to "dnssec" is they are.

maddy does not validate DNSSEC signatures on its own. Instead it reslies on
the upstream resolver to do so by causing lookup to fail when verification
fails and setting the AD flag for signed and verfified zones. As a safety
measure, if the resolver is not 127.0.0.1 or ::1, the AD flag is ignored.

DNSSEC is currently not supported on Windows and other platforms that do not
have the /etc/resolv.conf file in the standard format.

```
dnssec { }
```

### DANE

Checks TLSA records for the recipient MX. Provides downgrade-resistant TLS
enforcement.

Sets TLS level to "authenticated" if a valid and matching TLSA record uses
DANE-EE or DANE-TA usage type.

See above for notes on DNSSEC. DNSSEC support is required for DANE to work.

```
dane { }
```

### Local policy

Checks effective TLS and MX levels (as set by other policies) against local
configuration.

```
local_policy {
	min_tls_level none
	min_mx_level none
}
```

Using 'local\_policy off' is equivalent to setting both directives to 'none'.

**Syntax**: min\_tls\_level none|encrypted|authenticated <br>
**Default**: none

Set the minimal TLS security level required for all outbound messages.

See [Security levels](../../seclevels) page for details.

**Syntax**: min\_mx\_level: none|mtasts|dnssec <br>
**Default**: none

Set the minimal MX security level required for all outbound messages.

See [Security levels](../../seclevels) page for details.

