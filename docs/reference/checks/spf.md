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

If `enforce_early` is `no`, check.spf module will not take any action on SPF
policy failure if sender domain does have a DMARC record with 'quarantine' or
'reject' policy. Instead it will rely on DMARC support to take necesary
actions using SPF results as an input.

Disabling `enforce_early` without enabling DMARC support will make SPF policies
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

### debug _boolean_
Default: global directive value

Enable verbose logging for check.spf.

---

### enforce_early _boolean_
Default: `no`

Make policy decision on MAIL FROM stage (before the message body is received).
This makes it impossible to apply DMARC override (see above).

---

### none_action `reject` | `quarantine` | `ignore`
Default: `ignore`

Action to take when SPF policy evaluates to a 'none' result.

See [https://tools.ietf.org/html/rfc7208#section-2.6](https://tools.ietf.org/html/rfc7208#section-2.6) for meaning of
SPF results.

---

### neutral_action `reject` | `quarantine` | `ignore`
Default: `ignore`

Action to take when SPF policy evaluates to a 'neutral' result.

See [https://tools.ietf.org/html/rfc7208#section-2.6](https://tools.ietf.org/html/rfc7208#section-2.6) for meaning of
SPF results.

---

### fail_action `reject` | `quarantine` | `ignore`
Default: `quarantine`

Action to take when SPF policy evaluates to a 'fail' result.

---

### softfail_action `reject` | `quarantine` | `ignore`
Default: `ignore`

Action to take when SPF policy evaluates to a 'softfail' result.

---

### permerr_action `reject` | `quarantine` | `ignore`
Default: `reject`

Action to take when SPF policy evaluates to a 'permerror' result.

---

### temperr_action `reject` | `quarantine` | `ignore`
Default: `reject`

Action to take when SPF policy evaluates to a 'temperror' result.
