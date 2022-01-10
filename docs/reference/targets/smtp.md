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

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging.

**Syntax**: tls\_client { ... } <br>
**Default**: not specified

Advanced TLS client configuration options. See [TLS configuration / Client](/reference/tls/#client) for details.

**Syntax**: attempt\_starttls _boolean_ <br>
**Default**: yes (no for target.lmtp)

Attempt to use STARTTLS if it is supported by the remote server.
If TLS handshake fails, connection will be retried without STARTTLS
unless 'require\_tls' is also specified.

**Syntax**: require\_tls _boolean_ <br>
**Default**: no

Refuse to pass messages over plain-text connections.

**Syntax**: <br>
auth off <br>
plain _username_ _password_ <br>
forward <br>
external <br>
**Default**: off

Specify the way to authenticate to the remote server.
Valid values:

- off

  No authentication.

- plain

  Authenticate using specified username-password pair.
  **Don't use** this without enforced TLS ('require\_tls').

- forward

  Forward credentials specified by the client.
  **Don't use** this without enforced TLS ('require\_tls').

- external

  Request "external" SASL authentication. This is usually used for
  authentication using TLS client certificates. See [TLS configuration / Client](/reference/tls/#client) for details.

**Syntax**: targets _endpoints..._ <br>
**Default:** not specified

REQUIRED.

List of remote server addresses to use. See [Address definitions](/reference/config-syntax/#address-definitions)
for syntax to use.  Basically, it is 'tcp://ADDRESS:PORT'
for plain SMTP and 'tls://ADDRESS:PORT' for SMTPS (aka SMTP with Implicit
TLS).

Multiple addresses can be specified, they will be tried in order until connection to
one succeeds (including TLS handshake if TLS is required).

**Syntax**: connect\_timeout _duration_ <br>
**Default**: 5m

Same as for target.remote.

**Syntax**: command\_timeout _duration_ <br>
**Default**: 5m

Same as for target.remote.

**Syntax**: submission\_timeout _duration_ <br>
**Default**: 12m

Same as for target.remote.