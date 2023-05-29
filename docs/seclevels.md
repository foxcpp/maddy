# Outbound delivery security

maddy implements a number of schemes and protocols for discovery and
enforcement of security features supported by the recipient MTA.

## Introduction to the problems of secure SMTP

Outbound delivery security involves two independent problems:

- MX record authentication
- TLS enforcement

### MX record authentication

When MTA wants to deliver a message to a mailbox at remote domain, it needs to
discover the server to use for it. It is done through the lookup of DNS MX
records for the recipient.

Problem arises from the fact that DNS does not have any cryptographic
protection and so any malicious actor can technically modify the response to
contain any server. And MTA would use that server!

There are two protocols that solve this problem: MTA-STS and DNSSEC.
Former requires the MTA to verify used records against a list of rules published
via HTTPS. Later cryptographically signs the records themselves.

### TLS enforcement

By default, server-server SMTP is unencrypted. If remote server supports TLS,
it is advertised via the ESMTP extension named STARTTLS, but malicious actor
controlling communication channel can hide the support for STARTTLS and sender
MTA will have to use plaintext. There needs to be a out-of-band authenticated
channel to indicate TLS support (and to require its use).

MTA-STS and DANE solve this problem. In the first case, if policy is in
"enforce" mode then MTA is required to use TLS when delivering messages to a
remote server. DANE does pretty much the same thing, but using DNSSEC-signed
TLSA records.

## maddy policy details

maddy defines two values indicating how "secure" delivery of message will be:

- MX security level
- TLS security level

These values correspond to the problems described above. On delivery, the
established connection to the remote server is "ranked" using these values and
then they are compared against a number of policies (including local
configuration). If the effective value is lower than the required one, the
connection is closed and next candidate server is used. If all connections fail
this way - the delivery is failed (or deferred if there was a temporary error
when checking policies).

Below is the table summarizing the security level values defined in maddy and
protection they offer.

| MX/TLS level  | None | Encrypted | Authenticated        |
| ------------- | ---- | --------- | -------------------- |
|     None      |  -   |    P      |      P               |
|    MTA-STS    |  -   |    P      |      PA (see note 1) |
|    DNSSEC     |  -   |    P      |      PA              |

Legend: P - protects against passive attacks; A - protects against active
attacks

- MX level: None. MX candidate was returned as a result of DNS lookup for the
  recipient domain, no additional checks done.
- MX level: MTA-STS. Used MX matches the MTA-STS policy published by the
  recipient domain (even one in testing mode).
- MX level: DNSSEC. MX record is signed.

- TLS level: None. Plaintext connection was established, TLS is not available
  or failed.
- TLS level: Encrypted. TLS connection was established, the server certificate
  failed X.509 and DANE verification.
- TLS level: Authenticated. TLS connection was established, the server
  certificate passes X.509 **or** DANE verification.

**Note 1:** Persistent attacker able to control network connection can
interfere with policy refresh, downgrading protection to be secure only against
passive attacks.

## maddy security policies

See [Remote MX delivery](/reference/targets/remote/) for description of configuration options available for each policy mechanism
supported by maddy.

[RFC 8461 Section 10.2]: https://www.rfc-editor.org/rfc/rfc8461.html#section-10.2 (SMTP MTA Strict Transport Security - 10.2. Preventing Policy Discovery)
