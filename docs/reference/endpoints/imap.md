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