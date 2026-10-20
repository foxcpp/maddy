# Ignore delivery errors

Module that wraps another delivery target and turns any delivery error into a
logged warning instead of propagating it.

This is useful for non-critical `deliver_to` copies (e.g. push notifications)
where a failure should not abort the rest of the pipeline.

```
deliver_to ignore_error smtp tcp://127.0.0.1:2525
```

The wrapped target is given inline, exactly as it would be written after
`deliver_to`. Any configuration block attached to the directive belongs to the
wrapped target and is forwarded as-is:

```
deliver_to ignore_error smtp tcp://127.0.0.1:2525 {
    connect_timeout 5s
}
```

Errors from every delivery stage of the wrapped target (connection, RCPT, body
and commit) are logged and then discarded, so the message is always reported as
delivered to the pipeline. If the wrapped target cannot be started at all, the
whole delivery becomes a no-op.
