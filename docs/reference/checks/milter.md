# Milter client

The 'milter' implements subset of Sendmail's milter protocol that can be used
to integrate external software with maddy.
maddy implements version 6 of the protocol, older versions are
not supported.

Notable limitations of protocol implementation in maddy include:
1. Changes of envelope sender address are not supported
2. Removal and addition of envelope recipients is not supported
3. Removal and replacement of header fields is not supported
4. Headers fields can be inserted only on top
5. Milter does not receive some "macros" provided by sendmail.

Restrictions 1 and 2 are inherent to the maddy checks interface and cannot be
removed without major changes to it. Restrictions 3, 4 and 5 are temporary due to
incomplete implementation.

```
check.milter {
	endpoint <endpoint>
	fail_open false
}

milter <endpoint>
```

## Arguments

When defined inline, the first argument specifies endpoint to access milter
via. See below.

## Configuration directives

### endpoint _scheme://path_
Default: not set

Specifies milter protocol endpoint to use.
The endpoit is specified in standard URL-like format:
`tcp://127.0.0.1:6669` or `unix:///var/lib/milter/filter.sock`

---

### fail_open _boolean_
Default: `false`

Toggles behavior on milter I/O errors. If false ("fail closed") - message is
rejected with temporary error code. If true ("fail open") - check is skipped.

