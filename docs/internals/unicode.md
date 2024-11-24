# Unicode support

maddy has the first-class Unicode support in all components (modules). You do
not have to take any actions to make it work with internationalized domains,
mailbox names or non-ASCII message headers.

Internally, all text fields in maddy are represented in UTF-8 and handled using
Unicode-aware operations for comparisons, case-folding and so on.

## Non-ASCII data in message headers and bodies

maddy SMTP implementation does not care about encodings used in MIME headers or
in `Content-Type text/*` charset field.

However, local IMAP storage implementation needs to perform certain operations
on header contents. This is mostly about SEARCH functionality. For IMAP search
to work correctly, the message body and headers should use one of the following
encodings:

- ASCII
- UTF-8
- ISO-8859-1, 2, 3, 4, 9, 10, 13, 14, 15 or 16
- Windows-1250, 1251 or 1252 (aka Code Page 1250 and so on)
- KOI8-R
- ~~HZGB2312~~, GB18030
- GBK (aka Code Page 936)
- Shift JIS (aka Code Page 932 or Windows-31J)
- Big-5 (aka Code Page 950)
- EUC-JP
- ISO-2022-JP

_Support for HZGB2312 is currently disabled due to bugs with security
implications._

If mailbox includes a message with any encoding not listed here, it will not
be returned in search results for any request.

Behavior regarding handling of non-Unicode encodings is not considered stable
and may change between versions (including removal of supported encodings). If
you need your stuff to work correctly - start using UTF-8.

## Configuration files

maddy configuration files are assumed to be encoded in UTF-8. Use of any other
encoding will break stuff, do not do it.

Domain names (e.g. in hostname directive or pipeline rules) can be represented
using the ACE form (aka Punycode). They will be converted to the Unicode form
internally.

## Local credentials

'sql' storage backend and authentication provider enforce a number of additional
constraints on used account names.

PRECIS UsernameCaseMapped profile is enforced for local email addresses.
It limits the use of control and Bidi characters to make sure the used value
can be represented consistently in a variety of contexts. On top of that, the
address is case-folded and normalized to the NFC form for consistent internal
handling.

PRECIS OpaqueString profile is enforced for passwords. Less strict rules are
applied here. Runs of Unicode whitespace characters are replaced with a single
ASCII space. NFC normalization is applied afterwards. If the resulting string
is empty - the password is not accepted.

Both profiles are defined in RFC 8265, consult it for details.

## Protocol support

### SMTPUTF8 extension

maddy SMTP implementation includes support for the SMTPUTF8 extension as
defined in RFC 6531.

This means maddy can handle internationalized mailbox and domain names in MAIL
FROM, RCPT TO commands both for outbound and inbound delivery.

maddy will not accept messages with non-ASCII envelope addresses unless
SMTPUTF8 support is requested. If a message with SMTPUTF8 flag set is forwarded
to a server without SMTPUTF8 support, delivery will fail unless it is possible
to represent envelope addresses in the ASCII form (only domains use Unicode and
they can be converted to Punycode). Contents of message body (and header) are
not considered and always accepted and sent as-is, no automatic downgrading or
reencoding is done.

### IMAP UTF8, I18NLEVEL extensions

Currently, maddy does not include support for UTF8 and I18NLEVEL IMAP
extensions. However, it is not a problem that can prevent it from correctly
handling UTF-8 messages (or even messages in other non-ASCII encodings
mentioned above).

Clients that want to implement proper handling for Unicode strings may assume
maddy does not handle them properly in e.g. SEARCH commands and so such clients
may download messages and process them locally.
