# Misc checks

## Configuration directives

Following directives are defined for all modules listed below.

### fail_action `ignore` | `reject` | `quarantine`
Default: `quarantine`

Action to take when check fails. See [Check actions](../actions/) for details.

---

### debug _boolean_
Default: global directive value

Log both successful and unsuccessful check executions instead of just
unsuccessful.

---

### require_mx_record

Check that domain in MAIL FROM command does have a MX record and none of them
are "null" (contain a single dot as the host).

By default, quarantines messages coming from servers missing MX records,
use `fail_action` directive to change that.

---

### require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing
PTR record, use `fail_action` directive to change that.

---

### require_tls

Check that the source server is connected via TLS; either directly, or by using
the STARTTLS command.

By default, rejects messages coming from unencrypted servers. Use the
`fail_action` directive to change that.