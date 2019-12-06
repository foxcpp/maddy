# Implementation quirks

This page documents unusual behavior of the maddy protocols implementations.
Some of these problems break standards, some don't but still can hurt
interoperability.

## SMTP

- `for` field is never included in the `Received` header field.

  This is allowed by [RFC 2821].

## IMAP

### `sql`

- `\Recent` flag is not implemented and it always set.

  This _does not_ break [RFC 3501]. Clients relying on it will work (much) less
  efficiently.

- Sequence numbers don't stay consistent between SELECT/CHECK commands.

  This _does not_ break [RFC 3501] which is unclear about synchronization
  issues, however it deviates from behavior implemented by most servers. This
  can lead to operations applied to the wrong messages if sequence numbers are
  used by multiple clients connected at the same time.

[RFC 2821]: https://tools.ietf.org/html/rfc2821
[RFC 3501]: https://tools.ietf.org/html/rfc3501
