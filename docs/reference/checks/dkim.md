# DKIM

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

## Configuration directives

```
check.dkim {
    debug no
    required_fields From Subject
    allow_body_subset no
    no_sig_action ignore
    broken_sig_action ignore
	fail_open no
}
```

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Log both successfull and unsuccessful check executions instead of just
unsuccessful.

**Syntax**: required\_fields _string..._ <br>
**Default**: From Subject

Header fields that should be included in each signature. If signature
lacks any field listed in that directive, it will be considered invalid.

Note that From is always required to be signed, even if it is not included in
this directive.

**Syntax**: no\_sig\_action _action_ <br>
**Default**: ignore (recommended by RFC 6376)

Action to take when message without any signature is received.

Note that DMARC policy of the sender domain can request more strict handling of
missing DKIM signatures.

**Syntax**: broken\_sig\_action _action_ <br>
**Default**: ignore (recommended by RFC 6376)

Action to take when there are not valid signatures in a message.

Note that DMARC policy of the sender domain can request more strict handling of
broken DKIM signatures.

**Syntax**: fail\_open _boolean_ <br>
**Default**: no

Whether to accept the message if a temporary error occurs during DKIM
verification. Rejecting the message with a 4xx code will require the sender
to resend it later in a hope that the problem will be resolved.
