maddy-filters(5) "maddy mail server" "maddy reference documentation"

; TITLE Message filtering

maddy does have two distinct types of modules that do message filtering.
"Checks" and "modifiers".

"Checks" are meant to be used to reject or quarantine
messages that are unwanted, such as potential spam or messages with spoofed
sender address. They are limited in ways they can modify the message and their
execution is heavily parallelized to improve performance.

"Modifiers" are not executed in parallel and are allowed to modify the message
in any way.

As a convention, most of modules listed here don't require a separate
configuration block explicitly and have implicit configuration block defined
with the name as a module itself. That is, you don't have to write this:
```
check {
    verify_dkim { }
}
```
or this:
```
verify_dkim {
    # empty
}

somewhere {
    check {
        verify_dkim
    }
}
```
If default check behavior works fine for you. You can just use module name to
reference its default config:
```
check {
    verify_dkim
}
```

# Check actions

When a certain check module thinks the message is "bad", it takes some actions
depending on its configuration. Most checks follow the same configuration
structure and allow following actions to be taken on check failure:

- Do nothing ('action ignore')

Useful for testing deployment of new checks. Check failures are still logged
but they have no effect on message delivery.

- Reject the message ('action reject')

Reject the message at connection time. No bounce is generated locally.

- Quarantine the message ('action quarantine')

Mark message as 'quarantined'. If message is then delivered to the local
storage, the storage backend can place the message in the 'Junk' mailbox.
Another thing to keep in mind that 'remote' module (see *maddy-targets*(5))
will refuse to send quarantined messages.

# DNS checks

## Configuration directives

Following directives are defined for all modules listed below.

*Syntax*: ++
    fail_action ignore ++
    fail_action reject ++
    fail_action quarantine ++
*Default*: quarantine

Action to take when check fails. See Check actions for details.

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Log both sucessfull and unsucessfull check executions instead of just
unsucessfull.

## require_matching_ehlo

Check that source server hostname (from EHLO/HELO command) resolves to source
server IP.

By default, quarantines messages coming from servers with mismatched
EHLO hostname, use 'fail_action' directive to change that.

## require_mx_record

Check that domain in MAIL FROM command does have a MX record.

By default, quarantines messages coming from servers missing MX records,
use 'fail_action' directive to change that.

## require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing
PTR record, use 'fail_action' directive to change that.

# DKIM authorization module (verify_dkim)

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

```
verify_dkim {
    debug no
    required_fields From Subject
    allow_body_subset no
    no_sig_action ignore
    broken_Sig_action ignore
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Log both sucessfull and unsucessfull check executions instead of just
unsucessfull.

*Syntax*: required_fields _string..._ ++
*Default*: From Subject

Header fields that should be included in each signature. If signature
lacks any field listed in that directive, it will be considered invalid.

Note that From is always required to be signed, even if it is not included in
this directive.

*Syntax*: allow_body_subset _boolean_ ++
*Default*: no

By default signatures that cover only body subset (using l= "body limit" tag)
are considered invalid ("broken"). This directive allows such signatures.

It should be noted that allowing use of "body limit" allows arbitrary
data to be appended to the message body.

*Syntax*: no_sig_action _action_ ++
*Default*: ignore (recommended by RFC 6376)

Action to take when message without any signature is received.

Note that DMARC policy of the sender domain can request more strict handling of
missing DKIM signatures.

*Syntax*: broken_sig_action _action_ ++
*Default*: ignore (recommended by RFC 6376)

Action to take when there are not valid signatures in a message.

Note that DMARC policy of the sender domain can request more strict handling of
broken DKIM signatures.

# Sender/recipient replacement modules (replace_sender, replace_rcpt)

These are modules that simply replace matching address value(s) with another
in either MAIL FROM or RCPT TO.

Matching is done either by full address string or regexp that should match
entire address (it is implicitly wrapped with ^ and $). In either case,
matching is case-insensitive.

Configuration is done using inline arguments or 'from' and 'to'
directives. See below for examples.

```
modify {
    # Replace addr@example.com with addr@example.org in MAIL FROM (message
    # sender).
    replace_sender addr@example.com addr@example.org

    # Replace addr@example.com with addr@example.org in RCPT TO (message
    # recipient).
    replace_rcpt addr@example.com addr@example.org

    # Examples below use replace_sender but work exactly the same way for
    # replace_rcpt.

    # Replace any address matching /-enclosed regexp with com@example.org.
    replace_sender /(.+)@example.com/ com@example.org

    # You can also reference capture groups in the second argument.
    replace_sender /(.+)@example.com/ $1@example.org
}
```
