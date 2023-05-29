# Mailman 3

Setting up Mailman 3 with maddy involves some additional work as compared to
other MTAs as there is no Python package in Mailman suite that can generate
address lists in format supported by maddy.

We assume you are already familiar with Mailman configuration guidelines and
how stuff works in general/for other MTAs.

## Accepting messages

First of all, you need to use NullMTA package for mta.incoming so Mailman will
not try to generate any configs. LMTP listener is configured as usual.
```
[mta]
incoming: mailman.mta.null.NullMTA
lmtp_host: 127.0.0.1
lmtp_port: 8024
```

After that, you will need to configure maddy to send messages to Mailman.

The preferable way of doing so is destination_in and table.regexp:
```
msgpipeline local_routing {
    destination_in regexp "first-mailinglist(-(bounces\+.*|confirm\+.*|join|leave|owner|request|subscribe|unsubscribe))?@lists.example.org" {
        deliver_to lmtp tcp://127.0.0.1:8024
    }
    destination_in regexp "second-mailinglist(-(bounces\+.*|confirm\+.*|join|leave|owner|request|subscribe|unsubscribe))?@lists.example.org" {
        deliver_to lmtp tcp://127.0.0.1:8024
    }

    ...
}
```

A more simple option is also meaningful (provided you have a separate domain
for lists):
```
msgpipeline local_routing {
    destination lists.example.org {
        deliver_to lmtp tcp://127.0.0.1:8024
    }

    ...
}
```
But this variant will lead to inefficient handling of non-existing subaddresses.
See [Mailman Core issue 14](https://gitlab.com/mailman/mailman/-/issues/14) for
details. (5 year old issue, sigh...)

## Sending messages

It is recommended to configure Mailman to send messages using Submission port
with authentication and TLS as maddy does not allow relay on port 25 for local
clients as some MTAs do:
```
[mta]
# ... incoming configuration here ...
outgoing: mailman.mta.deliver.deliver
smtp_host: mx.example.org
smtp_port: 465
smtp_user: mailman@example.org
smtp_pass: something-very-secret
smtp_secure_mode: smtps
```

If you do not want to use TLS and/or authentication you can create a separate
endpoint and just point Mailman to it. E.g.
```
smtp tcp://127.0.0.1:2525 {
    destination postmaster $(local_domains) {
        deliver_to &local_routing
    }
    default_destination {
        deliver_to &remote_queue
    }
}
```

Note that if you use a separate domain for lists, it need to be included in
local_domains macro in default config. This will ensure maddy signs messages
using DKIM for outbound messages. It is also highly recommended to configure
ARC in Mailman 3.
