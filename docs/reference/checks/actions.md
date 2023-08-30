# Check actions

When a certain check module thinks the message is "bad", it takes some actions
depending on its configuration. Most checks follow the same configuration
structure and allow following actions to be taken on check failure:

- Do nothing (`action ignore`)

Useful for testing deployment of new checks. Check failures are still logged
but they have no effect on message delivery.

- Reject the message (`action reject`)

Reject the message at connection time. No bounce is generated locally.

- Quarantine the message (`action quarantine`)

Mark message as 'quarantined'. If message is then delivered to the local
storage, the storage backend can place the message in the 'Junk' mailbox.
Another thing to keep in mind that 'target.remote' module
will refuse to send quarantined messages.