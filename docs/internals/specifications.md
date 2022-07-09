# Followed specifications

This page lists Internet Standards and other specifications followed by
maddy along with any known deviations.


## Message format

- [RFC 2822] - Internet Message Format
- [RFC 2045] - Multipurpose Internet Mail Extensions (MIME) (part 1)
- [RFC 2046] - Multipurpose Internet Mail Extensions (MIME) (part 2)
- [RFC 2047] - Multipurpose Internet Mail Extensions (MIME) (part 3)
- [RFC 2048] - Multipurpose Internet Mail Extensions (MIME) (part 4)
- [RFC 2049] - Multipurpose Internet Mail Extensions (MIME) (part 5)
- [RFC 6532] - Internationalized Email Headers

- [RFC 2183] - Communicating Presentation Information in Internet Messages: The
  Content-Disposition Header Field

## IMAP

- [RFC 3501] - Internet Message Access Protocol - Version 4rev1
    * **Partial**: `\Recent` flag is not reset sometimes.
- [RFC 2152] - UTF-7

### Extensions

- [RFC 2595] - Using TLS with IMAP, POP3 and ACAP
- [RFC 7889] - The IMAP APPENDLIMIT Extension
- [RFC 3348] - The Internet Message Action Protocol (IMAP4). Child Mailbox
  Extension
- [RFC 6851] - Internet Message Access Protocol (IMAP) - MOVE Extension
- [RFC 6154] - IMAP LIST Extension for Special-Use Mailboxes
    * **Partial**: Only SPECIAL-USE capability.
- [RFC 5255] - Internet Message Access Protocol Internationalization
    * **Partial**: Only I18NLEVEL=1 capability.
- [RFC 4978] - The IMAP COMPRESS Extension
- [RFC 3691] - Internet Message Access Protocol (IMAP) UNSELECT command
- [RFC 2177] - IMAP4 IDLE command
- [RFC 7888] - IMAP4 Non-Synchronizing Literals
    * LITERAL+ capability.
- [RFC 4959] - IMAP Extension for Simple Authentication and Security Layer
  (SASL) Initial Client Response

## SMTP

- [RFC 2033] - Local Mail Transfer Protocol
- [RFC 5321] - Simple Mail Transfer Protocol
- [RFC 6409] - Message Submission for Mail

### Extensions

- [RFC 1870] - SMTP Service Extension for Message Size Declaration
- [RFC 2920] - SMTP Service Extension for Command Pipelining
    * Server support only, not used by SMTP client
- [RFC 2034] - SMTP Service Extension for Returning Enhanced Error Codes
- [RFC 3207] - SMTP Service Extension for Secure SMTP over Transport Layer
  Security
- [RFC 4954] - SMTP Service Extension for Authentication
- [RFC 6152] - SMTP Extension for 8-bit MIME
- [RFC 6531] - SMTP Extension for Internationalized Email

### Misc

- [RFC 6522] - The Multipart/Report Content Type for the Reporting of Mail
  System Administrative Messages
- [RFC 3464] - An Extensible Message Format for Delivery Status Notifications
- [RFC 6533] - Internationalized Delivery Status and Disposition Notifications

## Email security

### User authentication

- [RFC 4422] - Simple Authentication and Security Layer (SASL)
- [RFC 4616] - The PLAIN Simple Authentication and Security Layer (SASL)
  Mechanism

### Sender authentication

- [RFC 6376] - DomainKeys Identified Mail (DKIM) Signatures
- [RFC 7001] - Message Header Field for Indicating Message Authentication Status
- [RFC 7208] - Sender Policy Framework (SPF) for Authorizing Use of Domains in
  Email, Version 1
- [RFC 7372] - Email Authentication Status Codes
- [RFC 7479] - Domain-based Message Authentication, Reporting, and Conformance
  (DMARC)
    * **Partial**: No report generation.
- [RFC 8301] - Cryptographic Algorithm and Key Usage Update to DomainKeys
  Identified Mail (DKIM)
- [RFC 8463] - A New Cryptographic Signature Method for DomainKeys Identified
  Mail (DKIM)
- [RFC 8616] - Email Authentication for Internationalized Mail

### Recipient authentication

- [RFC 4033] - DNS Security Introduction and Requirements
- [RFC 6698] - The DNS-Based Authentication of Named Entities (DANE) Transport
  Layer Security (TLS) Protocol: TLSA
- [RFC 7672] - SMTP Security via Opportunistic DNS-Based Authentication of
  Named Entities (DANE) Transport Layer Security (TLS)
- [RFC 8461] - SMTP MTA Strict Transport Security (MTA-STS)

## Unicode, encodings, internationalization

- [RFC 3492] - Punycode: A Bootstring encoding of Unicode for Internationalized
  Domain Names in Applications (IDNA)
- [RFC 3629] - UTF-8, a transformation format of ISO 10646
- [RFC 5890] - Internationalized Domain Names for Applications (IDNA):
  Definitions and Document Framework
- [RFC 5891] - Internationalized Domain Names for Applications (IDNA): Protocol
- [RFC 7616] - Preparation, Enforcement, and Comparison of Internationalized
  Strings Representing Usernames and Passwords
- [RFC 8264] - PRECIS Framework: Preparation, Enforcement, and Comparison of
  Internationalized Strings in Application Protocols
- [Unicode 11.0.0]
    - [UAX #15] - Unicode Normalization Forms

There is a huge list of non-Unicode encodings supported by message parser used
for IMAP static cache and search.  See [Unicode support](unicode.md) page for
details.

## Misc

- [RFC 5782] - DNS Blacklists and Whitelists


[GH 188]: https://github.com/foxcpp/maddy/issues/188

[RFC 2822]: https://tools.ietf.org/html/rfc2822
[RFC 2045]: https://tools.ietf.org/html/rfc2045
[RFC 2046]: https://tools.ietf.org/html/rfc2046
[RFC 2047]: https://tools.ietf.org/html/rfc2047
[RFC 2048]: https://tools.ietf.org/html/rfc2048
[RFC 2049]: https://tools.ietf.org/html/rfc2049
[RFC 6532]: https://tools.ietf.org/html/rfc6532
[RFC 2183]: https://tools.ietf.org/html/rfc2183
[RFC 3501]: https://tools.ietf.org/html/rfc3501
[RFC 2152]: https://tools.ietf.org/html/rfc2152
[RFC 2595]: https://tools.ietf.org/html/rfc2595
[RFC 7889]: https://tools.ietf.org/html/rfc7889
[RFC 3348]: https://tools.ietf.org/html/rfc3348
[RFC 6851]: https://tools.ietf.org/html/rfc6851
[RFC 6154]: https://tools.ietf.org/html/rfc6154
[RFC 5255]: https://tools.ietf.org/html/rfc5255
[RFC 4978]: https://tools.ietf.org/html/rfc4978
[RFC 3691]: https://tools.ietf.org/html/rfc3691
[RFC 2177]: https://tools.ietf.org/html/rfc2177
[RFC 7888]: https://tools.ietf.org/html/rfc7888
[RFC 4959]: https://tools.ietf.org/html/rfc4959
[RFC 2033]: https://tools.ietf.org/html/rfc2033
[RFC 5321]: https://tools.ietf.org/html/rfc5321
[RFC 6409]: https://tools.ietf.org/html/rfc6409
[RFC 1870]: https://tools.ietf.org/html/rfc1870
[RFC 2920]: https://tools.ietf.org/html/rfc2920
[RFC 2034]: https://tools.ietf.org/html/rfc2034
[RFC 3207]: https://tools.ietf.org/html/rfc3207
[RFC 4954]: https://tools.ietf.org/html/rfc4954
[RFC 6152]: https://tools.ietf.org/html/rfc6152
[RFC 6531]: https://tools.ietf.org/html/rfc6531
[RFC 6522]: https://tools.ietf.org/html/rfc6522
[RFC 3464]: https://tools.ietf.org/html/rfc3464
[RFC 6533]: https://tools.ietf.org/html/rfc6533
[RFC 4422]: https://tools.ietf.org/html/rfc4422
[RFC 4616]: https://tools.ietf.org/html/rfc4616
[RFC 6376]: https://tools.ietf.org/html/rfc6376
[RFC 7001]: https://tools.ietf.org/html/rfc7001
[RFC 7208]: https://tools.ietf.org/html/rfc7208
[RFC 7372]: https://tools.ietf.org/html/rfc7372
[RFC 7479]: https://tools.ietf.org/html/rfc7479
[RFC 8301]: https://tools.ietf.org/html/rfc8301
[RFC 8463]: https://tools.ietf.org/html/rfc8463
[RFC 8616]: https://tools.ietf.org/html/rfc8616
[RFC 4033]: https://tools.ietf.org/html/rfc4033
[RFC 6698]: https://tools.ietf.org/html/rfc6698
[RFC 7672]: https://tools.ietf.org/html/rfc7672
[RFC 8461]: https://tools.ietf.org/html/rfc8461
[RFC 3492]: https://tools.ietf.org/html/rfc3492
[RFC 3629]: https://tools.ietf.org/html/rfc3629
[RFC 5890]: https://tools.ietf.org/html/rfc5890
[RFC 5891]: https://tools.ietf.org/html/rfc5891
[RFC 7616]: https://tools.ietf.org/html/rfc7616
[RFC 8264]: https://tools.ietf.org/html/rfc8264
[RFC 5782]: https://tools.ietf.org/html/rfc5782
[RFC 2822]: https://tools.ietf.org/html/rfc2822
[RFC 2045]: https://tools.ietf.org/html/rfc2045
[RFC 2046]: https://tools.ietf.org/html/rfc2046
[RFC 2047]: https://tools.ietf.org/html/rfc2047
[RFC 2048]: https://tools.ietf.org/html/rfc2048
[RFC 2049]: https://tools.ietf.org/html/rfc2049
[RFC 6532]: https://tools.ietf.org/html/rfc6532
[RFC 3501]: https://tools.ietf.org/html/rfc3501
[RFC 2595]: https://tools.ietf.org/html/rfc2595
[RFC 7889]: https://tools.ietf.org/html/rfc7889
[RFC 3348]: https://tools.ietf.org/html/rfc3348
[RFC 6851]: https://tools.ietf.org/html/rfc6851
[RFC 6154]: https://tools.ietf.org/html/rfc6154
[RFC 5255]: https://tools.ietf.org/html/rfc5255
[RFC 4978]: https://tools.ietf.org/html/rfc4978
[RFC 3691]: https://tools.ietf.org/html/rfc3691
[RFC 2177]: https://tools.ietf.org/html/rfc2177
[RFC 7888]: https://tools.ietf.org/html/rfc7888
[RFC 4959]: https://tools.ietf.org/html/rfc4959
[RFC 2033]: https://tools.ietf.org/html/rfc2033
[RFC 5321]: https://tools.ietf.org/html/rfc5321
[RFC 6409]: https://tools.ietf.org/html/rfc6409
[RFC 1870]: https://tools.ietf.org/html/rfc1870
[RFC 2920]: https://tools.ietf.org/html/rfc2920
[RFC 2034]: https://tools.ietf.org/html/rfc2034
[RFC 3207]: https://tools.ietf.org/html/rfc3207
[RFC 4954]: https://tools.ietf.org/html/rfc4954
[RFC 6152]: https://tools.ietf.org/html/rfc6152
[RFC 6531]: https://tools.ietf.org/html/rfc6531
[RFC 6522]: https://tools.ietf.org/html/rfc6522
[RFC 3464]: https://tools.ietf.org/html/rfc3464
[RFC 6533]: https://tools.ietf.org/html/rfc6533
[RFC 4422]: https://tools.ietf.org/html/rfc4422
[RFC 4616]: https://tools.ietf.org/html/rfc4616
[RFC 6376]: https://tools.ietf.org/html/rfc6376
[RFC 7001]: https://tools.ietf.org/html/rfc7001
[RFC 7208]: https://tools.ietf.org/html/rfc7208
[RFC 7372]: https://tools.ietf.org/html/rfc7372
[RFC 7479]: https://tools.ietf.org/html/rfc7479
[RFC 8301]: https://tools.ietf.org/html/rfc8301
[RFC 8463]: https://tools.ietf.org/html/rfc8463
[RFC 8616]: https://tools.ietf.org/html/rfc8616
[RFC 4033]: https://tools.ietf.org/html/rfc4033
[RFC 6698]: https://tools.ietf.org/html/rfc6698
[RFC 7672]: https://tools.ietf.org/html/rfc7672
[RFC 8461]: https://tools.ietf.org/html/rfc8461
[RFC 3492]: https://tools.ietf.org/html/rfc3492
[RFC 3629]: https://tools.ietf.org/html/rfc3629
[RFC 5890]: https://tools.ietf.org/html/rfc5890
[RFC 5891]: https://tools.ietf.org/html/rfc5891
[RFC 7616]: https://tools.ietf.org/html/rfc7616
[RFC 8264]: https://tools.ietf.org/html/rfc8264
[RFC 5782]: https://tools.ietf.org/html/rfc5782
[RFC 2822]: https://tools.ietf.org/html/rfc2822
[RFC 2045]: https://tools.ietf.org/html/rfc2045
[RFC 2046]: https://tools.ietf.org/html/rfc2046
[RFC 2047]: https://tools.ietf.org/html/rfc2047
[RFC 2048]: https://tools.ietf.org/html/rfc2048
[RFC 2049]: https://tools.ietf.org/html/rfc2049
[RFC 6532]: https://tools.ietf.org/html/rfc6532
[RFC 3501]: https://tools.ietf.org/html/rfc3501
[RFC 2595]: https://tools.ietf.org/html/rfc2595
[RFC 7889]: https://tools.ietf.org/html/rfc7889
[RFC 3348]: https://tools.ietf.org/html/rfc3348
[RFC 6851]: https://tools.ietf.org/html/rfc6851
[RFC 6154]: https://tools.ietf.org/html/rfc6154
[RFC 5255]: https://tools.ietf.org/html/rfc5255
[RFC 4978]: https://tools.ietf.org/html/rfc4978
[RFC 3691]: https://tools.ietf.org/html/rfc3691
[RFC 2177]: https://tools.ietf.org/html/rfc2177
[RFC 7888]: https://tools.ietf.org/html/rfc7888
[RFC 4959]: https://tools.ietf.org/html/rfc4959
[RFC 2033]: https://tools.ietf.org/html/rfc2033
[RFC 5321]: https://tools.ietf.org/html/rfc5321
[RFC 6409]: https://tools.ietf.org/html/rfc6409
[RFC 1870]: https://tools.ietf.org/html/rfc1870
[RFC 2920]: https://tools.ietf.org/html/rfc2920
[RFC 2034]: https://tools.ietf.org/html/rfc2034
[RFC 3207]: https://tools.ietf.org/html/rfc3207
[RFC 4954]: https://tools.ietf.org/html/rfc4954
[RFC 6152]: https://tools.ietf.org/html/rfc6152
[RFC 6531]: https://tools.ietf.org/html/rfc6531
[RFC 6522]: https://tools.ietf.org/html/rfc6522
[RFC 3464]: https://tools.ietf.org/html/rfc3464
[RFC 6533]: https://tools.ietf.org/html/rfc6533
[RFC 4422]: https://tools.ietf.org/html/rfc4422
[RFC 4616]: https://tools.ietf.org/html/rfc4616
[RFC 6376]: https://tools.ietf.org/html/rfc6376
[RFC 8301]: https://tools.ietf.org/html/rfc8301
[RFC 8463]: https://tools.ietf.org/html/rfc8463
[RFC 7208]: https://tools.ietf.org/html/rfc7208
[RFC 7372]: https://tools.ietf.org/html/rfc7372
[RFC 7479]: https://tools.ietf.org/html/rfc7479
[RFC 8616]: https://tools.ietf.org/html/rfc8616
[RFC 4033]: https://tools.ietf.org/html/rfc4033
[RFC 6698]: https://tools.ietf.org/html/rfc6698
[RFC 7672]: https://tools.ietf.org/html/rfc7672
[RFC 8461]: https://tools.ietf.org/html/rfc8461
[RFC 3492]: https://tools.ietf.org/html/rfc3492
[RFC 3629]: https://tools.ietf.org/html/rfc3629
[RFC 5890]: https://tools.ietf.org/html/rfc5890
[RFC 5891]: https://tools.ietf.org/html/rfc5891
[RFC 7616]: https://tools.ietf.org/html/rfc7616
[RFC 8264]: https://tools.ietf.org/html/rfc8264
[RFC 5782]: https://tools.ietf.org/html/rfc5782

[Unicode 11.0.0]: https://www.unicode.org/versions/components-11.0.0.html
[UAX #15]: https://unicode.org/reports/tr15/
