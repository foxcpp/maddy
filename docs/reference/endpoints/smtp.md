# SMTP/LMTP/Submission endpoint

Module 'smtp' is a listener that implements ESMTP protocol with optional
authentication, LMTP and Submission support. Incoming messages are processed in
accordance with pipeline rules (explained in Message pipeline section below).

```
smtp tcp://0.0.0.0:25 {
    hostname example.org
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    io_debug no
    debug no
    insecure_auth no
    sasl_login no
    read_timeout 10m
    write_timeout 1m
    max_message_size 32M
    max_header_size 1M
    auth pam
    defer_sender_reject yes
    dmarc yes
    smtp_max_line_length 4000
    limits {
        endpoint rate 10
        endpoint concurrency 500
    }

    # Example pipeline ocnfiguration.
    destination example.org {
        deliver_to &local_mailboxes
    }
    default_destination {
        reject
    }
}
```

## Configuration directives

### hostname _string_
Default: global directive value

Server name to use in SMTP banner.

```
220 example.org ESMTP Service Ready
```

---

### tls _certificate-path_ _key-path_ { ... }
Default: global directive value

TLS certificate & key to use. Fine-tuning of other TLS properties is possible
by specifying a configuration block and options inside it:

```
tls cert.crt key.key {
    protocols tls1.2 tls1.3
}
```

See [TLS configuration / Server](/reference/tls/#server-side) for details.

---

### proxy_protocol _trusted ips..._ { ... } <br>
Default: not enabled

Enable use of HAProxy PROXY protocol. Supports both v1 and v2 protocols.
If a list of trusted IP addresses or subnets is provided, only connections
from those will be trusted.

TLS for the channel between the proxies and maddy can be configured
using a 'tls' directive:
```
proxy_protocol {
    trust 127.0.0.1 ::1 192.168.0.1/24
    tls &proxy_tls
}
```

---

### io_debug _boolean_
Default: `no`

Write all commands and responses to stderr.

---

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### insecure_auth _boolean_
Default: `no` (`yes` if TLS is disabled)

Allow plain-text authentication over unencrypted connections. Not recommended!

---

### sasl_login _boolean_
Default: `no`

Enable support for SASL LOGIN authentication mechanism used by
some outdated clients.

---

### read_timeout _duration_
Default: `10m`

I/O read timeout.

---

### write_timeout _duration_
Default: `1m`

I/O write timeout.

---

### max_message_size _size_
Default: `32M`

Limit the size of incoming messages to 'size'.

---

### max_header_size _size_
Default: `1M`

Limit the size of incoming message headers to 'size'.

---

### auth _module-reference_
Default: not specified

Use the specified module for authentication.

---

### defer_sender_reject _boolean_
Default: `yes`

Apply sender-based checks and routing logic when first RCPT TO command
is received. This allows maddy to log recipient address of the rejected
message and also improves interoperability with (improperly implemented)
clients that don't expect an error early in session.

---

### max_logged_rcpt_errors _integer_
Default: `5`

Amount of RCPT-time errors that should be logged. Further errors will be
handled silently. This is to prevent log flooding during email dictionary
attacks (address probing).

---

### max_received _integer_
Default: `50`

Max. amount of Received header fields in the message header. If the incoming
message has more fields than this number, it will be rejected with the permanent error
5.4.6 ("Routing loop detected").

---

### buffer `ram`<br>buffer `fs` _path_ <br>buffer `auto` _max-size_ _path_
Default: `auto 1M StateDirectory/buffer`

Temporary storage to use for the body of accepted messages.

- `ram` – Store the body in RAM.
- `fs` – Write out the message to the FS and read it back as needed.
_path_ can be omitted and defaults to StateDirectory/buffer.
- `auto` – Store message bodies smaller than `_max_size_` entirely in RAM,
otherwise write them out to the FS. _path_ can be omitted and defaults to `StateDirectory/buffer`.

---

### smtp_max_line_length _integer_
Default: `4000`

The maximum line length allowed in the SMTP input stream. If client sends a
longer line - connection will be closed and message (if any) will be rejected
with a permanent error.

RFC 5321 has the recommended limit of 998 bytes. Servers are not required
to handle longer lines correctly but some senders may produce them.

Unless BDAT extension is used by the sender, this limitation also applies to
the message body.

---

### dmarc _boolean_
Default: `yes`

Enforce sender's DMARC policy. Due to implementation limitations, it is not a
check module.

**Note**: Report generation is not implemented now.

**Note**: DMARC needs SPF and DKIM checks to function correctly.
Without these, DMARC check will not run.

---

## Rate & concurrency limiting

### limits { ... }
Default: no limits

This allows configuring a set of message flow restrictions including
max. concurrency and rate per-endpoint, per-source, per-destination.

Limits are specified as directives inside the block:

```
limits {
	all rate 20
	destination concurrency 5
}
```

Supported limits:

### _scope_ rate _burst_ _period_

Rate limit. Restrict the amount of messages processed in _period_ to
_burst_ messages. If period is not specified, 1 second is used.

### _scope_ concurrency _max_
Concurrency limit. Restrict the amount of messages processed in parallel
to _max_.

For each supported limitation, _scope_ determines whether it should be applied
for all messages ("all"), per-sender IP ("ip"), per-sender domain ("source") or
per-recipient domain ("destination"). Having a scope other than "all" means
that the restriction will be enforced independently for each group determined
by scope. E.g.  "ip rate 20" means that the same IP cannot send more than 20
messages per second. "destination concurrency 5" means that no more than 5
messages can be sent in parallel to a single domain.

**Note**: At the moment, SMTP endpoint on its own does not support per-recipient
limits.  They will be no-op. If you want to enforce a per-recipient restriction
on outbound messages, do so using 'limits' directive for the 'table.remote' module

It is possible to share limit counters between multiple endpoints (or any other
modules). To do so define a top-level configuration block for module "limits"
and reference it where needed using standard & syntax. E.g.

```
limits inbound_limits {
	all rate 20
}

smtp smtp://0.0.0.0:25 {
	limits &inbound_limits
	...
}

submission tls://0.0.0.0:465 {
	limits &inbound_limits
	...
}
```

Using an "all rate" restriction in such way means that no more than 20
messages can enter the server through both endpoints in one second.

# Submission module (submission)

Module 'submission' implements all functionality of the 'smtp' module and adds
certain message preprocessing on top of it, additionally authentication is
always required.

'submission' module checks whether addresses in header fields From, Sender, To,
Cc, Bcc, Reply-To are correct and adds Message-ID and Date if it is missing.

```
submission tcp://0.0.0.0:587 tls://0.0.0.0:465 {
    # ... same as smtp ...
}
```

# LMTP module (lmtp)

Module 'lmtp' implements all functionality of the 'smtp' module but uses
LMTP (RFC 2033) protocol.

```
lmtp unix://lmtp.sock {
    # ... same as smtp ...
}
```

## Limitations of LMTP implementation

- Can't be used with TCP.
- Delivery to 'sql' module storage is always atomic, either all recipients will
  succeed or none of them will.

