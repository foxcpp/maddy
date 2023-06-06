# Dovecot

Builtin maddy IMAP server may not match your requirements in terms of
performance, reliability or anything. For this reason it is possible to
integrate it with any external IMAP server that implements necessary
protocols. Here is how to do it for Dovecot.

1. Get rid of `imap` endpoint and existing `local_authdb` and `local_mailboxes`
   blocks.

2. Setup Dovecot to provide LMTP endpoint

Here is an example configuration snippet:
```
# /etc/dovecot/dovecot.conf
protocols = imap lmtp

# /etc/dovecot/conf.d/10-master.conf
service lmtp {
 unix_listener lmtp-maddy {
   mode = 0600
   user = maddy
  }
}
```

Add `local_mailboxes` block to maddy config using `target.lmtp` module:
```
target.lmtp local_mailboxes {
    targets unix:///var/run/dovecot/lmtp-maddy
}
```

### Authentication

In addition to MTA service, maddy also provides Submission service, but it
needs authentication provider data to work correctly, maddy can use Dovecot
SASL authentication protocol for it.

You need the following in Dovecot's `10-master.conf`:
```
service auth {
  unix_listener auth-maddy-client {
    mode = 0660
    user = maddy
  }
}
```

Then just configure `dovecot_sasl` module for `submission`:
```
submission ... {
    auth dovecot_sasl unix:///var/run/dovecot/auth-maddy-client
    ... other configuration ...
}
```

## Other IMAP servers

Integration with other IMAP servers might be more problematic because there is
no standard protocol for authentication delegation. You might need to configure
the IMAP server to implement MSA functionality by forwarding messages to maddy
for outbound delivery. This might require more configuration changes on maddy
side since by default it will not allow relay on port 25 even for localhost
addresses. The easiest way is to create another SMTP endpoint on some port
(probably Submission port):
```
smtp tcp://127.0.0.1:587 {
    deliver_to &remote_queue
}
```
And configure IMAP server's Submission service to forward outbound messages
there.

Depending on how Submission service is implemented you may also need to route
messages for local domains back to it via LMTP:
```
smtp tcp://127.0.0.1:587 {
    destination postmaster $(local_domains) {
        deliver_to &local_routing
    }
    default_destination {
        deliver_to &remote_queue
    }
}
```

