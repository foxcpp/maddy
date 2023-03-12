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
    auth pam
    storage &local_mailboxes
    auth_map identity
    auth_map_normalize auto
    storage_map identity
    storage_map_normalize auto
}
```

**Syntax**: tls _certificate\_path_ _key\_path_ { ... } <br>
**Default**: global directive value

TLS certificate & key to use. Fine-tuning of other TLS properties is possible
by specifing a configuration block and options inside it:
```
tls cert.crt key.key {
    protocols tls1.2 tls1.3
}
```

See [TLS configuration / Server](/reference/tls/#server-side) for details.

**Syntax**: io\_debug _boolean_ <br>
**Default**: no

Write all commands and responses to stderr.

**Syntax**: io\_errors _boolean_ <br>
**Default**: no

Log I/O errors.

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging.

**Syntax**: insecure\_auth _boolean_ <br>
**Default**: no (yes if TLS is disabled)

**Syntax**: auth _module\_reference\_

Use the specified module for authentication.
**Required.**

**Syntax**: storage _module\_reference\_

Use the specified module for message storage.
**Required.**

**Syntax**: storage\_map _module\_reference_ <br>
**Default**: identity

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

**Syntax**: storage\_map_normalize _function_ <br>
**Default**: auto

Same as `auth_map_normalize` but for `storage_map`.

**Syntax**: auth\_map_normalize _function_ <br>
**Default**: auto

Overrides global `auth_map_normalize` value for this endpoint.

See [Global configuration](/reference/global-config) for details.



