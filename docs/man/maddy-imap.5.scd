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
    storage &local_mailboxes
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

## IMAP filters

Most storage backends support application of custom code late in delivery
process. As opposed to using SMTP pipeline modifiers or checks, it allows
modifying IMAP-specific message attributes. In particular, it allows
code to change target folder and add IMAP flags (keywords) to the message.

There is no way to reject message using IMAP filters, this should be done
eariler in SMTP pipeline logic. Quarantined messages are not processed
by IMAP filters and are unconditionally delivered to Junk folder (or other
folder with \Junk special-use attribute).

## System command filter (imap.filter.command)

This filter is similar to check.command module described in *maddy-filters*(5)
and runs system command

```
command executable_name args... { }
```

Same as check.command, following placeholders are supported for command
arguments: {source_ip}, {source_host}, {source_rdns}, {msg_id}, {auth_user},
{sender}.

Additionally, for imap.filter.command, {account_name} placeholder is replaced
with effective IMAP account name.

Command cutput consists of zero or more lines. First one, if non-empty, overrides
destination folder. All other lines contain additional IMAP flags to add
to the message. If command wants to add flags without changing folder - first
line should be empty.

It is valid for command to not write anything to stdout. In this case its
execution will have no effect on delivery.
