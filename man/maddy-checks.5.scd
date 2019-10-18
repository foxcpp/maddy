maddy.conf(5) "maddy mail server - message checks"

# INTRODUCTION

This man page lists "message check" modules. "Checks" are meant to be used to reject
or quarantine messages that are unwanted, such as potential spam or messages with spoofed
sender address.

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
If default check behavior works fine for you. You can just use module name to reference
its default config:
```
check {
    verify_dkim
} 
```

# SEE ALSO

*maddy.conf*(5) for main man page.

# ACTIONS & SCORING

When a certain check module thinks the message is "bad", it takes some actions depending
on its configuration. Most checks follow the same configuration structure and allow
following actions to be taken on check failure:
- Do nothing ('action ignore')

Useful for testing deployment of new checks. Check failures are still logged 
but they have no effect on message delivery.

- Reject the message ('action reject')

Reject the message at connection time. No bounce is generated locally.

- Quarantine the message ('action quarantine')

Mark message as 'quarantined'. If message is then delivered to the local storage, the storage
backend can place the message in the 'Junk' mailbox. Another thing to keep in mind that
'remote' module (see maddy-targets(5)) will refuse to send quarantined messages.

- Adjust message score

Message score is a integer that defines how likely the message is unwanted. Using message
scores instead of direct actions (as above) allows to combine decisions of various checks
and allow checks to override results of other checks.

After all checks are executed, the message score is compared against two values.
First one is 'quarantine score', if message score is higher than that value, it will 
be quarantined (see above). Second one is 'reject score', if message score is higher than 
that value, message will be rejected at connection time.  These values are configured
using 'quarantine_score' and 'reject_score' directives in 'smtp' module 
configuration (see maddy.conf(5)). Both values are undefined by default and so are
not used.

# COMMON DIRECTIVES

All check modules have 'debug' directive to enable debug logging, just like most of maddy modules.

Here is the exact syntax of 'action'-like directives:

- action ignore

Ignore the check failure, useful for testing deployments to see
how useful it is.

- action reject

Reject the message instead of quarantining it.

- action quarantine

Quarantine the message, this is the default for most checks.

- action score <number>

Increase checks score by <number> (can be negative to decrease it).

Additionally, most checks define the following configuration directive:

- ok_score <number>

Increase checks score by <number> if check passed (can be negative to decrease it).

# DNS CHECKS

## require_matching_ehlo

Check that source server hostname (from EHLO/HELO command) resolves to source
server IP.

By default, quarantines messages coming from servers with mismatched 
EHLO hostname, use fail_action to change that.

## require_mx_record

Check that domain in MAIL FROM command does have a MX record.

By default, quarantines messages coming from servers missing MX records,
use fail_action to change that.

## require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing PTR record,
use fail_action to change that.

# VERIFY_DKIM MODULE

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

It got an implicit configuration block defined like DNS checks above.


Example, to quarantine messages with invalid signatures (not a good idea, DKIM breaks 
sometimes for weird reasons):
```
check {
    ...
    verify_dkim {
        broken_sig_action qurantine
    }
}
```

Valid directives:

## debug [yes/no]

Toggle debug logging only for this module.

## required_fields <fields...>

Header fields that should be included in each signature. If signature
lacks any field listed in that directive, it will be considered invalid ("broken").
Authentication-Results will contain 

Default is 'From Subject'.

Note that From is always required to be signed, even if it is not
included in this directive.  

## no_sig_action ...

Action to take when message without any signature is received.
Default is 'ignore', as recommended by RFC 6376.

## broken_sig_action ...

Action to take when message with invalid signature is received.
Default is 'ignore', as recommended by RFC 6376.
