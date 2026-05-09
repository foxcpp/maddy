# IMAP4rev2 endpoint (EXPERIMENTAL)

Module 'imap2' is a listener that implements IMAP4rev2 protocol and provides
access to local messages storage specified by the 'storage' directive.
The endpoint is compatible with both IMAP4rev1 and IMAP4rev2 clients.

**Currently, imap2 endpoint is considered EXPERIMENTAL and some parts of
functionality can be missing e.g. imap2 does not support server-side
sorting or threading yet.**

imap2 endpoint requires the use of a compatible storage backend e.g. corresponding
imapsql2 storage module. imap2 can be used with IMAP4rev1 storage modules, but
some protocol features may be further limited.

In most cases, local storage modules will auto-create accounts when they are
accessed via IMAP. This relies on authentication provider used by IMAP endpoint
to provide what essentially is access control. There is a caveat, however: this
auto-creation will not happen when delivering incoming messages via SMTP as
there is no authentication to confirm that this account should indeed be
created.

## Configuration directives

```
imap2 tls://0.0.0.0:993 tcp://0.0.0.0:143 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    io_debug no
    io_errors yes
    proxy_protocol ...
    debug no
    insecure_auth no
    sasl_login no
    auth pam
    storage &local_mailboxes
    auth_map identity
    auth_map_normalize auto
    storage_map identity
    storage_map_normalize auto
}
```

### tls _certificate-path_ _key-path_ { ... }
Default: global directive value

TLS certificate & key to use. Fine-tuning of other TLS properties is possible
by specifying a configuration block and options inside it:

```
tls cert.crt key.key {
    protocols tls1.2 tls1.3
}
```

See [TLS configuration / Server](/reference/tls/#server-side) for details.

---

### proxy_protocol _trusted ips..._ { ... }
Default: not enabled

Enable use of HAProxy PROXY protocol. Supports both v1 and v2 protocols.
If a list of trusted IP addresses or subnets is provided, only connections
from those will be trusted.

TLS for the channel between the proxies and maddy can be configured
using a 'tls' directive:
```
proxy_protocol {
    trust 127.0.0.1 ::1 192.168.0.1/24
    tls &proxy_tls
}
```
Note that the top-level 'tls' directive is not inherited here. If you
need TLS on top of the PROXY protocol, securing the protocol header,
you must declare TLS explicitly.

---

### io_debug _boolean_
Default: `no`

Write all IMAP commands and responses to stderr.

**WARNING:** This will log authentication data, including plaintext passwords
if clients use IMAP LOGIN mechanism. Use only for debugging, never in production.

---

### io_errors _boolean_
Default: `yes`

Log low-level I/O errors and connection issues.

---

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### insecure_auth _boolean_
Default: `no` (`yes` if TLS is disabled)

Allow plain-text authentication over unencrypted connections. This is disabled
by default and should only be enabled if you understand the security implications.
The server will warn when this is enabled without TLS.

---

### sasl_login _boolean_
Default: `no`

Enable support for SASL LOGIN authentication mechanism used by some outdated clients.

**NOTE:** SASL LOGIN is deprecated and should only be enabled for compatibility
with legacy clients.

---

### auth _module-reference_
**Required.**

Use the specified module for authentication. The authentication provider determines
which users can connect and which mailboxes they can access.

---

### storage _module-reference_
**Required.**

Use the specified module for message storage and mailbox operations.

---

### storage_map _module-reference_
Default: `identity`

Use the specified table to map SASL usernames to storage account names.

Before username is looked up, it is normalized using function defined by
`storage_map_normalize`.

This directive is useful if you want users user@example.org and user@example.com
to share the same storage account named "user". In this case, use

```
    storage_map email_localpart
```

Note that `storage_map` does not affect the username passed to the
authentication provider.

It also does not affect how message delivery is handled, you should specify
`delivery_map` in storage module to define how to map email addresses
to storage accounts. E.g.

```
    storage.imapsql2 local_mailboxes {
        ...
        delivery_map email_localpart # deliver "user@*" to mailbox for "user"
    }
```

---

### storage_map_normalize _function_
Default: `auto`

Normalization function to use for `storage_map`.

See [Global configuration](/reference/global-config) for details, including
valid values.

---

### auth_map _module-reference_
Default: `identity`

Use the specified table to map login usernames before passing them to the
authentication provider.

Before username is looked up, it is normalized using function defined by
`auth_map_normalize`.

---

### auth_map_normalize _function_
Default: `auto`

Normalization function to use for `auth_map`.

Overrides global `auth_map_normalize` value for this endpoint.

See [Global configuration](/reference/global-config) for details, including
valid values.
