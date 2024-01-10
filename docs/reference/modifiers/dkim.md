# DKIM signing

modify.dkim module is a modifier that signs messages using DKIM
protocol (RFC 6376).

Each configuration block specifies a single selector
and one or more domains.

A key will be generated or read for each domain, the key to use
for each message will be selected based on the SMTP envelope sender. Exception
for that is that for domain-less postmaster address and null address, the
key for the first domain will be used. If domain in envelope sender
does not match any of loaded keys, message will not be signed.
Additionally, for each messages From header is checked to 
match MAIL FROM and authorization identity (username sender is logged in as).
This can be controlled using require_sender_match directive.

Generated private keys are stored in unencrypted PKCS#8 format
in state_directory/dkim_keys (`/var/lib/maddy/dkim_keys`).
In the same directory .dns files are generated that contain
public key for each domain formatted in the form of a DNS record.

## Arguments

domains and selector can be specified in arguments, so actual modify.dkim use can
be shortened to the following:

```
modify {
    dkim example.org selector
}
```

## Configuration directives

```
modify.dkim {
    debug no
    domains example.org example.com
    selector default
    key_path dkim-keys/{domain}-{selector}.key
    oversign_fields ...
    sign_fields ...
    header_canon relaxed
    body_canon relaxed
    sig_expiry 120h # 5 days
    hash sha256
    newkey_algo rsa2048
}
```

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### domains _string-list_
**Required**. <br>
Default: not specified


ADministrative Management Domains (ADMDs) taking responsibility for messages.

Should be specified either as a directive or as an argument.

---

### selector _string_
**Required**. <br>
Default: not specified

Identifier of used key within the ADMD.
Should be specified either as a directive or as an argument.

---

### key_path _string_
Default: `dkim_keys/{domain}_{selector}.key`

Path to private key. It should be in PKCS#8 format wrapped in PAM encoding.
If key does not exist, it will be generated using algorithm specified
in newkey_algo.

Placeholders '{domain}' and '{selector}' will be replaced with corresponding
values from domain and selector directives.

Additionally, keys in PKCS#1 ("RSA PRIVATE KEY") and
RFC 5915 ("EC PRIVATE KEY") can be read by modify.dkim. Note, however that
newly generated keys are always in PKCS#8.

---

### oversign_fields _list..._
Default: see below

Header fields that should be signed n+1 times where n is times they are
present in the message. This makes it impossible to replace field
value by prepending another field with the same name to the message.

Fields specified here don't have to be also specified in `sign_fields`.

Default set of oversigned fields:

- Subject
- To
- From
- Date
- MIME-Version
- Content-Type
- Content-Transfer-Encoding
- Reply-To
- Message-Id
- References
- Autocrypt
- Openpgp

---

### sign_fields _list..._
Default: see below

Header fields that should be signed n times where n is times they are
present in the message. For these fields, additional values can be prepended
by intermediate relays, but existing values can't be changed.

Default set of signed fields:

- List-Id
- List-Help
- List-Unsubscribe
- List-Post
- List-Owner
- List-Archive
- Resent-To
- Resent-Sender
- Resent-Message-Id
- Resent-Date
- Resent-From
- Resent-Cc

---

### header_canon `relaxed` | `simple`
Default: `relaxed`

Canonicalization algorithm to use for header fields. With `relaxed`, whitespace within
fields can be modified without breaking the signature, with `simple` no
modifications are allowed.

---

### body_canon `relaxed` | `simple`
Default: `relaxed`

Canonicalization algorithm to use for message body. With `relaxed`, whitespace within
can be modified without breaking the signature, with `simple` no
modifications are allowed.

---

### sig_expiry _duration_
Default: `120h`

Time for which signature should be considered valid. Mainly used to prevent
unauthorized resending of old messages.

---

### hash _hash_
Default: `sha256`

Hash algorithm to use when computing body hash.

sha256 is the only supported algorithm now.

---

### newkey_algo `rsa4096` | `rsa2048` | `ed25519`
Default: `rsa2048`

Algorithm to use when generating a new key.

Currently ed25519 is **not** supported by most platforms.

---

### require_sender_match _ids..._
Default: `envelope auth`

Require specified identifiers to match From header field and key domain,
otherwise - don't sign the message.

If From field contains multiple addresses, message will not be
signed unless `allow_multiple_from` is also specified. In that
case only first address will be compared.

Matching is done in a case-insensitive way.

Valid values:

- `off` – Disable check, always sign.
- `envelope` – Require MAIL FROM address to match From header.
- `auth` – If authorization identity contains @ - then require it to
  fully match From header. Otherwise, check only local-part
  (username).

---

### allow_multiple_from _boolean_
Default: `no`

Allow multiple addresses in From header field for purposes of
`require_sender_match` checks. Only first address will be checked, however.

---

### sign_subdomains _boolean_
Default: `no`

Sign emails from subdomains using a top domain key.

Allows only one domain to be specified (can be worked around by using `modify.dkim`
multiple times).
