# SMTP & LMTP transparent forwarding

Module that implements transparent forwarding of messages over SMTP.

Use in pipeline configuration:

```
deliver_to smtp tcp://127.0.0.1:5353
# or
deliver_to smtp tcp://127.0.0.1:5353 {
  # Other settings, see below.
}
```

target.lmtp can be used instead of target.smtp to
use LMTP protocol.

Endpoint addresses use format described in [Configuration files syntax / Address definitions](/reference/config-syntax/#address-definitions).

## Configuration directives

```
target.smtp {
    debug no
    tls_client {
        ...
    }
    attempt_starttls yes
    require_tls no
    auth off
    targets tcp://127.0.0.1:2525
    connect_timeout 5m
    command_timeout 5m
    submission_timeout 12m
}
```

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### tls_client { ... }
Default: not specified

Advanced TLS client configuration options. See [TLS configuration / Client](/reference/tls/#client) for details.

---

### starttls _boolean_
Default: `yes` (`no` for `target.lmtp`)

Use STARTTLS to enable TLS encryption. If STARTTLS is not supported
by the remote server - connection will fail.

maddy will use `localhost` as HELO hostname before STARTTLS
and will only send its actual hostname after STARTTLS.

### attempt_starttls _boolean_
Default: `yes` (`no` for `target.lmtp`)

DEPRECATED: Equivalent to `starttls`. Plaintext fallback is no longer
supported.

---

### require_tls _boolean_
Default: `no`

DEPRECATED: Ignored. Set `starttls yes` to use STARTLS.

---

### auth `off` | `plain` _username_ _password_ | `forward`  | `external`
Default: `off`

Specify the way to authenticate to the remote server.
Valid values:

- `off` – No authentication.
- `plain` – Authenticate using specified username-password pair.
  **Don't use** this without enforced TLS (`require_tls`).
- `forward` – Forward credentials specified by the client.
  **Don't use** this without enforced TLS (`require_tls`).
- `external` – Request "external" SASL authentication. This is usually used for
  authentication using TLS client certificates. See [TLS configuration / Client](/reference/tls/#client) for details.

---

### targets _endpoints..._
**Required.**<br>
Default: not specified

List of remote server addresses to use. See [Address definitions](/reference/config-syntax/#address-definitions)
for syntax to use.  Basically, it is `tcp://ADDRESS:PORT`
for plain SMTP and `tls://ADDRESS:PORT` for SMTPS (aka SMTP with Implicit
TLS).

Multiple addresses can be specified, they will be tried in order until connection to
one succeeds (including TLS handshake if TLS is required).

---

### connect_timeout _duration_
Default: `5m`

Same as for target.remote.

---

### command_timeout _duration_
Default: `5m`

Same as for target.remote.

---

### submission_timeout _duration_
Default: `12m`

Same as for target.remote.
