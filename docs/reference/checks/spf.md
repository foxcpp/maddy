# SPF

check.spf the check module that verifies whether IP address of the client is
authorized to send messages for domain in MAIL FROM address.

SPF statuses are mapped to maddy check actions in a way
specified by \*_action directives. By default, SPF failure 
results in the message being quarantined and errors (both permanent and 
temporary) cause message to be rejected.
Authentication-Results field is generated irregardless of status.

## DMARC override

It is recommended by the DMARC standard to don't fail delivery based solely on
SPF policy and always check DMARC policy and take action based on it.

If enforce\_early is no, check.spf module will not take any action on SPF
policy failure if sender domain does have a DMARC record with 'quarantine' or
'reject' policy. Instead it will rely on DMARC support to take necesary
actions using SPF results as an input.

Disabling enforce\_early without enabling DMARC support will make SPF policies
no-op and is considered insecure.

## Configuration directives

```
check.spf {
    debug no
    enforce_early no
    fail_action quarantine
    softfail_action ignore
    permerr_action reject
    temperr_action reject
}
```

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging for check.spf.

**Syntax**: enforce\_early _boolean_ <br>
**Default**: no

Make policy decision on MAIL FROM stage (before the message body is received).
This makes it impossible to apply DMARC override (see above).

**Syntax**: none\_action reject|qurantine|ignore <br>
**Default**: ignore

Action to take when SPF policy evaluates to a 'none' result.

See [https://tools.ietf.org/html/rfc7208#section-2.6](https://tools.ietf.org/html/rfc7208#section-2.6) for meaning of
SPF results.

**Syntax**: neutral\_action reject|qurantine|ignore <br>
**Default**: ignore

Action to take when SPF policy evaluates to a 'neutral' result.

See [https://tools.ietf.org/html/rfc7208#section-2.6](https://tools.ietf.org/html/rfc7208#section-2.6) for meaning of
SPF results.

**Syntax**: fail\_action reject|qurantine|ignore <br>
**Default**: quarantine

Action to take when SPF policy evaluates to a 'fail' result.

**Syntax**: softfail\_action reject|qurantine|ignore <br>
**Default**: ignore

Action to take when SPF policy evaluates to a 'softfail' result.

**Syntax**: permerr\_action reject|qurantine|ignore <br>
**Default**: reject

Action to take when SPF policy evaluates to a 'permerror' result.

**Syntax**: temperr\_action reject|qurantine|ignore <br>
**Default**: reject

Action to take when SPF policy evaluates to a 'temperror' result.
