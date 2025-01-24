# IMAP4rev1 endpoint

Module 'imap' is a listener that implements IMAP4rev1 protocol and provides
access to local messages storage specified by 'storage' directive.

In most cases, local storage modules will auto-create accounts when they are
accessed via IMAP. This relies on authentication provider used by IMAP endpoint
to provide what essentially is access control. There is a caveat, however: this
auto-creation will not happen when delivering incoming messages via SMTP as
there is no authentication to confirm that this account should indeed be
created.

## Configuration directives

```
imap tcp://0.0.0.0:143 tls://0.0.0.0:993 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    io_debug no
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

Write all commands and responses to stderr.

---

### io_errors _boolean_
Default: `no`

Log I/O errors.

---

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### insecure_auth _boolean_
Default: `no` (`yes` if TLS is disabled)

Allow plain-text authentication over unencrypted connections.

---

### sasl_login _boolean_
Default: `no`

Enable support for SASL LOGIN authentication mechanism used by
some outdated clients.

---

### auth _module-reference_
**Required.**

Use the specified module for authentication.

---

### storage _module-reference_
**Required.**

Use the specified module for message storage.

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
    storage.imapsql local_mailboxes {
        ...
        delivery_map email_localpart # deliver "user@*" to mailbox for "user"
    }
```

---

### storage_map_normalize _function_
Default: `auto`

Same as `auth_map_normalize` but for `storage_map`.

---

### auth_map_normalize _function_
Default: `auto`

Overrides global `auth_map_normalize` value for this endpoint.

See [Global configuration](/reference/global-config) for details.



