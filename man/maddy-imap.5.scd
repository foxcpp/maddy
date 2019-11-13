maddy-imap(5) "maddy mail server" "maddy reference documentation"

; TITLE IMAP endpoint module

Module 'imap' is a listener that implements IMAP4rev1 protocol and provides
access to local messages storage specified by 'storage' directive. See
*maddy-storage*(5) for support storage backends and corresponding
configuration options.

```
imap tcp://0.0.0.0:143 tls://0.0.0.0:993 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    io_debug no
    debug no
    insecure_auth no
    auth pam
    storage local_mailboxes
}
```

## Configuration directives

*Syntax*: tls _certificate_path_ _key_path_ { ... } ++
*Default*: global directive value

TLS certificate & key to use. Fine-tuning of other TLS properties is possible
by specifing a configuration block and options inside it:
```
tls cert.crt key.key {
    protocols tls1.2 tls1.3
}
```
See section 'TLS configuration' in *maddy*(1) for valid options.

*Syntax*: io_debug _boolean_ ++
*Default*: no

Write all commands and responses to stderr.

*Syntax*: io_errors _boolean_ ++
*Default*: no

Log I/O errors.

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging.

*Syntax*: insecure_auth _boolean_ ++
*Default*: no (yes if TLS is disabled)

*Syntax*: auth _module_reference_

Use the specified module for authentication.
*Required.*

*Syntax*: storage _module_reference_

Use the specified module for message storage.
*Required.*
