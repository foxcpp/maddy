maddy-filters(5) "maddy mail server" "maddy reference documentation"

; TITLE Message filtering

maddy does have two distinct types of modules that do message filtering.
"Checks" and "modifiers".

"Checks" are meant to be used to reject or quarantine
messages that are unwanted, such as potential spam or messages with spoofed
sender address. They are limited in ways they can modify the message and their
execution is heavily parallelized to improve performance.

"Modifiers" are executed serially in order they are referenced in the
configuration and are allowed to modify the message data and meta-data.

# Check actions

When a certain check module thinks the message is "bad", it takes some actions
depending on its configuration. Most checks follow the same configuration
structure and allow following actions to be taken on check failure:

- Do nothing ('action ignore')

Useful for testing deployment of new checks. Check failures are still logged
but they have no effect on message delivery.

- Reject the message ('action reject')

Reject the message at connection time. No bounce is generated locally.

- Quarantine the message ('action quarantine')

Mark message as 'quarantined'. If message is then delivered to the local
storage, the storage backend can place the message in the 'Junk' mailbox.
Another thing to keep in mind that 'remote' module (see *maddy-targets*(5))
will refuse to send quarantined messages.

# DNS checks

## Configuration directives

Following directives are defined for all modules listed below.

*Syntax*: ++
    fail_action ignore ++
    fail_action reject ++
    fail_action quarantine ++
*Default*: quarantine

Action to take when check fails. See Check actions for details.

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Log both sucessfull and unsucessfull check executions instead of just
unsucessfull.

## require_matching_ehlo

Check that source server hostname (from EHLO/HELO command) resolves to source
server IP.

By default, quarantines messages coming from servers with mismatched
EHLO hostname, use 'fail_action' directive to change that.

## require_mx_record

Check that domain in MAIL FROM command does have a MX record.

By default, quarantines messages coming from servers missing MX records,
use 'fail_action' directive to change that.

## require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing
PTR record, use 'fail_action' directive to change that.

# DKIM authentication module (verify_dkim)

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

```
verify_dkim {
    debug no
    required_fields From Subject
    allow_body_subset no
    no_sig_action ignore
    broken_sig_action ignore
	fail_open no
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Log both sucessfull and unsucessfull check executions instead of just
unsucessfull.

*Syntax*: required_fields _string..._ ++
*Default*: From Subject

Header fields that should be included in each signature. If signature
lacks any field listed in that directive, it will be considered invalid.

Note that From is always required to be signed, even if it is not included in
this directive.

*Syntax*: allow_body_subset _boolean_ ++
*Default*: no

By default signatures that cover only body subset (using l= "body limit" tag)
are considered invalid ("broken"). This directive allows such signatures.

It should be noted that allowing use of "body limit" allows arbitrary
data to be appended to the message body.

*Syntax*: no_sig_action _action_ ++
*Default*: ignore (recommended by RFC 6376)

Action to take when message without any signature is received.

Note that DMARC policy of the sender domain can request more strict handling of
missing DKIM signatures.

*Syntax*: broken_sig_action _action_ ++
*Default*: ignore (recommended by RFC 6376)

Action to take when there are not valid signatures in a message.

Note that DMARC policy of the sender domain can request more strict handling of
broken DKIM signatures.

*Syntax*: fail_open _boolean_ ++
*Default*: no

Whether to accept the message if a temporary error occurs during DKIM
verification. Rejecting the message with a 4xx code will require the sender
to resend it later in a hope that the problem will be resolved.

# SPF policy enforcement module (apply_spf)

This is the check module that verifies whether IP address of the client is
authorized to send messages for domain in MAIL FROM address.

```
apply_spf {
    debug no
    enforce_early no
    fail_action quarantine
    softfail_action quarantine
    permerr_action reject
    temperr_action reject
}
```

## DMARC override

It is recommended by the DMARC standard to don't fail delivery based solely on
SPF policy and always check DMARC policy and take action based on it.

If enforce_early is no, apply_spf module will not take any action on SPF
policy failure if sender domain does have a DMARC record with 'quarantine' or
'reject' policy. Instead it will rely on DMARC support to take necesary
actions using SPF results as an input.

Disabling enforce_early without enabling DMARC support will make SPF policies
no-op and is considered insecure.

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging for apply_spf.

*Syntax*: enforce_early _boolean_ ++
*Default*: no

Make policy decision on MAIL FROM stage (before the message body is received).
This makes it impossible to apply DMARC override (see above).

*Syntax*: none_action reject|qurantine|ignore ++
*Default*: ignore

Action to take when SPF policy evaluates to a 'none' result.

See https://tools.ietf.org/html/rfc7208#section-2.6 for meaning of
SPF results.

*Syntax*: neutral_action reject|qurantine|ignore ++
*Default*: ignore

Action to take when SPF policy evaluates to a 'neutral' result.

See https://tools.ietf.org/html/rfc7208#section-2.6 for meaning of
SPF results.

*Syntax*: fail_action reject|qurantine|ignore ++
*Default*: quarantine

Action to take when SPF policy evaluates to a 'fail' result.

*Syntax*: softfail_action reject|qurantine|ignore ++
*Default*: quarantine

Action to take when SPF policy evaluates to a 'softfail' result.

*Syntax*: permerr_action reject|qurantine|ignore ++
*Default*: reject

Action to take when SPF policy evaluates to a 'permerror' result.

*Syntax*: temperr_action reject|qurantine|ignore ++
*Default*: reject

Action to take when SPF policy evaluates to a 'temperror' result.

# DNSBL lookup module (dnsbl)

The dnsbl module implements checking of source IP and hostnames against a set
of DNS-based Blackhole lists (DNSBLs).

Its configuration consists of module configuration directives and a set
of blocks specifing lists to use and kind of lookups to perform on them.

```
dnsbl {
    debug no
    check_early no
    listed_action reject

    # Lists configuration example.
    dnsbl.example.org {
        client_ipv4 yes
        client_ipv6 no
        ehlo no
        mailfrom no
    }
    hsrbl.example.org {
        client_ipv4 no
        client_ipv6 no
        ehlo yes
        mailfrom yes
    }
}
```

## Arguments

Arguments specify the list of IP-based BLs to use.

The following configurations are equivalent.

```
check {
    dnsbl dnsbl.example.org dnsbl2.example.org
}
```

```
check {
    dnsbl {
        dnsbl.example.org dnsbl2.example.org {
            client_ipv4 yes
            client_ipv6 no
            ehlo no
            mailfrom no
        }
    }
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging.

*Syntax*: check_early _boolean_ ++
*Default*: no

Check BLs before mail delivery starts and silently reject blacklisted clients.

For this to work correctly, check should not be used in source/destination
pipeline block.

In particular, this means:
- No logging is done for rejected messages.
- listed_action takes no effect. It is always reject.
- defer_sender_reject from SMTP configuration takes no effect.
- MAIL FROM is not checked, even if specified.

If you often get hit by spam attacks, this is recommended to enable this
setting to save server resources.

*Syntax*: listed_action reject|quarantine|ignore ++
*Default*: reject

Action to take when one of the client identifiers is listed on the DNSBL.

## List configuration

```
dnsbl.example.org dnsbl.example.com {
    client_ipv4 yes
    client_ipv6 no
    ehlo no
    mailfrom no
    responses 127.0.0.1/24
    whitelist no
}
```

Directive name and arguments specify the actual DNS zone to query when checking
the list. Using multiple arguments is equivalent to specifying the same
configuration separately for each list.

*Syntax*: client_ipv4 _boolean_ ++
*Default*: yes

Whether to check address of the IPv4 clients against the list.

*Syntax*: client_ipv6 _boolean_ ++
*Default*: yes

Whether to check address of the IPv6 clients against the list.

*Syntax*: ehlo _boolean_ ++
*Default*: no

Whether to check hostname specified n the HELO/EHLO command
against the list.

This works correctly only with domain-based DNSBLs.

*Syntax*: mailfrom _boolean_ ++
*Default*: no

Whether to check domain part of the MAIL FROM address against the list.

This works correctly only with domain-based DNSBLs.

*Syntax*: responses _cidr|ip..._ ++
*Default*: 127.0.0.1/24

IP networks (in CIDR notation) or addresses to permit in list lookup results.
Addresses not matching any entry in this directives will be ignored.

*Syntax*: whitelist _boolean_ ++
*Default*: no

This list is a whitelist. If client IP is listed on it, all BL listings should
be ignored.

This directive can't be used with 'ehlo' and 'mailfrom' since these identifiers
are under sender control.

# DKIM signing module (sign_dkim)

sign_dkim module is a modifier that signs messages using DKIM
protocol (RFC 6376).

```
sign_dkim {
    debug no
    domain example.org
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

## Arguments

domain and selector can be specified in arguments, so actual sign_dkim use can
be shortened to the following:
```
modify {
    sign_dkim example.org selector
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging for sign_dkim.

*Syntax*: domain _string_ ++
*Default*: not specified

*REQUIRED.*

ADministrative Management Domain (ADMD) taking responsibility for signed
messages. Should be specified either as a directive or as an
argument.

*Syntax*: selector _string_ ++
*Default*: not specified

*REQUIRED.*

Identifier of used key within the ADMD.
Should be specified either as a directive or as an argument.

*Syntax*: key_path _string_ ++
*Default*: dkim_keys/{domain}\_{selector}.key

Path to private key. It should be in PKCS#8 format wrapped in PAM encoding.
If key does not exist, it will be generated using algorithm specified
in newkey_algo.

Placeholders '{domain}' and '{selector}' will be replaced with corresponding
values from domain and selector directives.

Additionally, keys in PKCS#1 ("RSA PRIVATE KEY") and
RFC 5915 ("EC PRIVATE KEY") can be read by sign_dkim. Note, however that
newly generated keys are always in PKCS#8.

*Syntax*: oversign_fields _list..._ ++
*Default*: see below

Header fields that should be signed n+1 times where n is times they are
present in the message. This makes it impossible to replace field
value by prepending another field with the same name to the message.

Fields specified here don't have to be also specified in sign_fields.

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

*Syntax*: sign_fields _list..._ ++
*Default*: see below

Header fields that should be signed n+1 times where n is times they are
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

*Syntax*: header_canon relaxed|simple ++
*Default*: relaxed

Canonicalization algorithm to use for header fields. With 'relaxed', whitespace within
fields can be modified without breaking the signature, with 'simple' no
modifications are allowed.

*Syntax*: body_canon relaxed|simple ++
*Default*: relaxed

Canonicalization algorithm to use for message body. With 'relaxed', whitespace within
can be modified without breaking the signature, with 'simple' no
modifications are allowed.

*Syntax*: sig_expiry _duration_ ++
*Default*: 120h

Time for which signature should be considered valid. Mainly used to prevent
unauthorized resending of old messages.

*Syntax*: hash _hash_ ++
*Default*: sha256

Hash algorithm to use when computing body hash.

sha256 is the only supported algorithm now.

*Syntax*: newkey_algo rsa4096|rsa2048|ed25519 ++
*Default*: rsa2048

Algorithm to use when generating a new key.

*Syntax*: require_sender_match _ids..._ ++
*Default*: envelope auth

Require specified identifiers to match From header field and key domain,
otherwise - don't sign the message.

If From field contains multiple addresses, message will not be
signed unless allow_multiple_from is also specified. In that
case only first address will be compared.

Matching is done in a case-insensitive way.

Valid values:
- off +
  Disable check, always sign.
- envelope +
  Require MAIL FROM address to match From header.
- auth +
  If authorization identity contains @ - then require it to
  fully match From header. Otherwise, check only local-part
  (username).

*Syntax*: allow_multiple_from _boolean_ ++
*Default*: no

Allow multiple addresses in From header field for purposes of
require_sender_match checks. Only first address will be checked, however.

# Sender/recipient replacement modules (replace_sender, replace_rcpt)

These are modules that simply replace matching address value(s) with another
in either MAIL FROM or RCPT TO.

Matching is done either by full address string or regexp that should match
entire address (it is implicitly wrapped with ^ and $). In either case,
matching is case-insensitive.

Configuration is done using arguments or 'from' and 'to'
directives. See below for examples.

```
modify {
    # Replace addr@example.com with addr@example.org in MAIL FROM (message
    # sender).
    replace_sender addr@example.com addr@example.org

    # Replace addr@example.com with addr@example.org in RCPT TO (message
    # recipient).
    replace_rcpt addr@example.com addr@example.org

    # Examples below use replace_sender but work exactly the same way for
    # replace_rcpt.

    # Replace any address matching /-enclosed regexp with com@example.org.
    replace_sender /(.+)@example.com/ com@example.org

    # You can also reference capture groups in the second argument.
    replace_sender /(.+)@example.com/ $1@example.org
}
```

# File-based recipient aliases (alias_file)

This module replaces recipient addresses based on the mapping defined
in the text file. Currently, only 1:1 mappings are supported (that is, it is
not possible to specify multiple replacements for a single address).

Matching is done case-insensitively. Name without '@' matches local-part with
any domain. Replacements are not applied recursively.

Recipients are not deduplicated after expansion, so message may be delivered
multiple times to a single recipient. However, used delivery target apply
such deduplication (sql does it).

```
alias_file {
    files ...
}
```

## Arguments

Specify the list of files to load aliases from.

## Auto-reloading

The module checks used files for changes every 15 seconds, if changes are
detected - all files are reloaded.

No changes are applied if any file contains syntax errors.

## File format

Similar to the format used by popular MTAs.

- Entry in the format:
  ```
  name: value
  ```

- Lines starting with '#', empty and whitespace-only lines are ignored.

Example:
```
# Replaces dog@domain to cat@domain for any domain.
dog: cat

# Replaces dog@domain to dog@example.com for any domain.
cat: dog@example.com

# Replaces cat@example.org to dog@example.org.
# Takes preference over any-domain alias above.
cat@example.org: dog@example.org

# Postmaster alias is a special case, it should
# always use a full address as a replacement.
postmaster: foobar@example.org

# Not allowed:
#postmaster: foobar
```

Following features usually implemented in other MTAs are currently not
supported in maddy:
```
cat: |command
cat: /file
cat: :include:file
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose log.

*Syntax*: files _path..._ ++
*Default*: not specified

List of files to read aliases from. Non-existent files are skipped.

At least one file should be specified either via directive or arguments.
