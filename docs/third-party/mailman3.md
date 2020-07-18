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
This need to be done for SMTP, Submission and bounces.

The preferrable way of doing so is destination_in and table.regexp:
```
(mailing_lists) {
    destination_in regexp "first-mailinglist(-(bounces\+.*|confirm\+.*|join|leave|owner|request|subscribe|unsubscribe))?@lists.example.org" {
        deliver_to lmtp tcp://127.0.0.1:8024
    }
    destination_in regexp "second-mailinglist(-(bounces\+.*|confirm\+.*|join|leave|owner|request|subscribe|unsubscribe))?@lists.example.org" {
        deliver_to lmtp tcp://127.0.0.1:8024
    }
}
```
or
```
(mailing_list) {
    destination_in regexp "first-mailinglist(-.*)?@lists.example.org" {
        deliver_to lmtp tcp://127.0.0.1:8024
    }
}
```
But second variant will lead to inefficient handling of non-existing subaddresses.
See [Mailman Core issue 14](https://gitlab.com/mailman/mailman/-/issues/14) for
details. (5 year old issue, sigh...)

After that, use mailing_lists 'snippet' in corresponding endpoint blocks:
```
# SMTP...
    default_source {
        import mailing_lists
        destination postmaster $(local_domains) {
            ...
        }
    }

# Submission...
    source $(local_domains) {
        import mailing_lists
        destination $(local_domains) {
            ...
        }
    }

# Queue...
    bounce {
        import mailing_lists
        destination $(local_domains) {
            ...
        }
        default_destination {
            ...
        }
    }
```

## Sending messages

It is recommended to configure Mailman to send messages using Submission port
with authentication and TLS as maddy does not allow relay on port 25 for local
users as some MTAs do:
```
outgoing: mailman.mta.deliver.deliver
smtp_host: mx.example.org
smtp_port: 465
smtp_user: mailman@example.org
smtp_pass: something-very-secret
smtp_secure_mode: smtps
```

If you do not want to use TLS and/or authentication you can create a separate
endpoint and just point Mailman to it.  E.g.
```
smtp tcp://127.0.0.1:2525 {
    destination $(local_domains) {
        modify &local_modifiers
        deliver_to &local_mailboxes
    }
    default_destination {
        modify &outbound_modifiers
        deliver_to &remote_queue
    }
}
```

Note that if you use a separate domain for lists, it need to be included in
local_domains macro in default config. This will ensure maddy signs messages
using DKIM for outbound messages. It is also highly recommended to configure
ARC in Mailman 3.
