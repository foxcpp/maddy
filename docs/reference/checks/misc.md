# Misc checks

## Configuration directives

Following directives are defined for all modules listed below.

**Syntax**: <br>
fail\_action ignore <br>
fail\_action reject <br>
fail\_action quarantine <br>
**Default**: quarantine

Action to take when check fails. See Check actions for details.

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Log both sucessfull and unsucessfull check executions instead of just
unsucessfull.

## require\_mx\_record

Check that domain in MAIL FROM command does have a MX record and none of them
are "null" (contain a single dot as the host).

By default, quarantines messages coming from servers missing MX records,
use 'fail\_action' directive to change that.

## require\_matching\_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing
PTR record, use 'fail\_action' directive to change that.

## require\_tls

Check that the source server is connected via TLS; either directly, or by using
the STARTTLS command.

By default, rejects messages coming from unencrypted servers. Use the
'fail\_action' directive to change that.