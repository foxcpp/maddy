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

# Simple checks

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

## require_mx_record

Check that domain in MAIL FROM command does have a MX record and none of them
are "null" (contain a single dot as the host).

By default, quarantines messages coming from servers missing MX records,
use 'fail_action' directive to change that.

## require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

By default, quarantines messages coming from servers with mismatched or missing
PTR record, use 'fail_action' directive to change that.

## require_tls

Check that the source server is connected via TLS; either directly, or by using
the STARTTLS command.

By default, rejects messages coming from unencrypted servers. Use the
'fail_action' directive to change that.

# DKIM authentication module (check.dkim)

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

```
check.dkim {
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

# SPF policy enforcement module (check.spf)

This is the check module that verifies whether IP address of the client is
authorized to send messages for domain in MAIL FROM address.

```
check.spf {
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

If enforce_early is no, check.spf module will not take any action on SPF
policy failure if sender domain does have a DMARC record with 'quarantine' or
'reject' policy. Instead it will rely on DMARC support to take necesary
actions using SPF results as an input.

Disabling enforce_early without enabling DMARC support will make SPF policies
no-op and is considered insecure.

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging for check.spf.

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

# DNSBL lookup module (check.dnsbl)

The dnsbl module implements checking of source IP and hostnames against a set
of DNS-based Blackhole lists (DNSBLs).

Its configuration consists of module configuration directives and a set
of blocks specifing lists to use and kind of lookups to perform on them.

```
check.dnsbl {
    debug no
    check_early no

    quarantine_threshold 1
    reject_threshold 1

    # Lists configuration example.
    dnsbl.example.org {
        client_ipv4 yes
        client_ipv6 no
        ehlo no
        mailfrom no
        score 1
    }
    hsrbl.example.org {
        client_ipv4 no
        client_ipv6 no
        ehlo yes
        mailfrom yes
        score 1
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
            score 1
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
- No action is taken if quarantine_threshold is hit, only reject_threshold
  applies.
- defer_sender_reject from SMTP configuration takes no effect.
- MAIL FROM is not checked, even if specified.

If you often get hit by spam attacks, this is recommended to enable this
setting to save server resources.

*Syntax*: quarantine_threshold _integer_ ++
*Default*: 1

DNSBL score needed (equals-or-higher) to quarantine the message.

*Syntax*: reject_threshold _integer_ ++
*Default*: 9999

DNSBL score needed (equals-or-higher) to reject the message.

## List configuration

```
dnsbl.example.org dnsbl.example.com {
    client_ipv4 yes
    client_ipv6 no
    ehlo no
    mailfrom no
    responses 127.0.0.1/24
	score 1
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

*Syntax*: score _integer_ ++
*Default*: 1

Score value to add for the message if it is listed.

If sum of list scores is equals or higher than quarantine_threshold, the
message will be quarantined.

If sum of list scores is equals or higher than rejected_threshold, the message
will be rejected.

It is possible to specify a negative value to make list act like a whitelist
and override results of other blocklists.

# DKIM signing module (modify.dkim)

sign_dkim module is a modifier that signs messages using DKIM
protocol (RFC 6376).

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

## Arguments

domains and selector can be specified in arguments, so actual sign_dkim use can
be shortened to the following:
```
modify {
    dkim example.org selector
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging for sign_dkim.

*Syntax*: domains _string list_ ++
*Default*: not specified

*REQUIRED.*

ADministrative Management Domains (ADMDs) taking responsibility for messages.

A key will be generated or read for each domain specified here, the key to use
for each message will be selected based on the SMTP envelope sender. Exception
for that is that for domain-less postmaster address and null address, the
key for the first domain will be used. If domain in envelope sender
does not match any of loaded keys, message will not be signed.

Should be specified either as a directive or as an argument.

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

*Syntax*: sign_subdomains _boolean_ ++
*Default*: no

Sign emails from subdomains using a top domain key.

Allows only one domain to be specified (can be workarounded using sign_dkim
multiple times).

# Envelope sender / recipient rewriting (modify.replace_sender, modify.replace_rcpt)

'replace_sender' and 'replace_rcpt' modules replace SMTP envelope addresses
based on the mapping defined by the table module (maddy-tables(5)). Currently,
only 1:1 mappings are supported (that is, it is not possible to specify
multiple replacements for a single address).

The address is normalized before lookup (Punycode in domain-part is decoded,
Unicode is normalized to NFC, the whole string is case-folded).

First, the whole address is looked up. If there is no replacement, local-part
of the address is looked up separately and is replaced in the address while
keeping the domain part intact. Replacements are not applied recursively, that
is, lookup is not repeated for the replacement.

Recipients are not deduplicated after expansion, so message may be delivered
multiple times to a single recipient. However, used delivery target can apply
such deduplication (imapsql storage does it).

Definition:
```
replace_rcpt <table> [table arguments] {
	[extended table config]
}
replace_sender <table> [table arguments] {
	[extended table config]
}
```

Use examples:
```
modify {
	replace_rcpt file_map /etc/maddy/aliases
	replace_rcpt static {
		entry a@example.org b@example.org
	}
	replace_rcpt regexp "(.+)@example.net" "$1@example.org"
}
```

Possible contents of /etc/maddy/aliases in the example above:
```
# Replace 'cat' with any domain to 'dog'.
# E.g. cat@example.net -> dog@example.net
cat: dog

# Replace cat@example.org with cat@example.com.
# Takes priority over the previous line.
cat@example.org: cat@example.com
```

# System command filter (check.command)

This module executes an arbitrary system command during a specified stage of
checks execution.

```
command executable_name arg0 arg1 ... {
	run_on body

	code 1 reject
	code 2 quarantine
}
```

## Arguments

The module arguments specify the command to run. If the first argument is not
an absolute path, it is looked up in the Libexec Directory (/usr/lib/maddy on
Linux) and in $PATH (in that ordering). Note that no additional handling
of arguments is done, especially, the command is executed directly, not via the
system shell.

There is a set of special strings that are replaced with the corresponding
message-specific values:

- {source_ip}

	IPv4/IPv6 address of the sending MTA.

- {source_host}

	Hostname of the sending MTA, from the HELO/EHLO command.

- {source_rdns}

	PTR record of the sending MTA IP address.

- {msg_id}

	Internal message identifier. Unique for each delivery.

- {auth_user}

	Client username, if authenticated using SASL PLAIN

- {sender}

	Message sender address, as specified in the MAIL FROM SMTP command.

- {rcpts}

	List of accepted recipient addresses, including the currently handled
	one.

- {address}

	Currently handled address. This is a recipient address if the command
	is called during RCPT TO command handling ('run_on rcpt') or a sender
	address if the command is called during MAIL FROM command handling ('run_on
	sender').


If value is undefined (e.g. {source_ip} for a message accepted over a Unix
socket) or unavailable (the command is executed too early), the placeholder
is replaced with an empty string. Note that it can not remove the argument.
E.g. -i {source_ip} will not become just -i, it will be -i ""

Undefined placeholders are not replaced.

## Command stdout

The command stdout must be either empty or contain a valid RFC 5322 header.
If it contains a byte stream that does not look a valid header, the message
will be rejected with a temporary error.

The header from stdout will be *prepended* to the message header.

## Configuration directives

*Syntax*: run_on conn|sender|rcpt|body ++
*Default*: body

When to run the command. This directive also affects the information visible
for the message.

- conn

	Run before the sender address (MAIL FROM) is handled.

	*Stdin*: Empty ++
*Available placeholders*: {source_ip}, {source_host}, {msg_id}, {auth_user}.

- sender

	Run during sender address (MAIL FROM) handling.

	*Stdin*: Empty ++
*Available placeholders*: conn placeholders + {sender}, {address}.

	The {address} placeholder contains the MAIL FROM address.

- rcpt

	Run during recipient address (RCPT TO) handling. The command is executed
	once for each RCPT TO command, even if the same recipient is specified
	multiple times.

	*Stdin*: Empty ++
*Available placeholders*: sender placeholders + {rcpts}.

	The {address} placeholder contains the recipient address.

- body

	Run during message body handling.

	*Stdin*: The message header + body ++
*Available placeholders*: all except for {address}.

*Syntax*: ++
	code _integer_ ignore ++
	code _integer_ quarantine ++
	code _integer_ reject [SMTP code] [SMTP enhanced code] [SMTP message]

This directives specified the mapping from the command exit code _integer_ to
the message pipeline action.

Two codes are defined implicitly, exit code 1 causes the message to be rejected
with a permanent error, exit code 2 causes the message to be quarantined. Both
action can be overriden using the 'code' directive.

## Milter protocol check (check.milter)

The 'milter' implements subset of Sendmail's milter protocol that can be used
to integrate external software in maddy.

Notable limitations of protocol implementation in maddy include:
1. Changes of envelope sender address are not supported
2. Removal and addition of envelope recipients is not supported
3. Removal and replacement of header fields is not supported
4. Headers fields can be inserted only on top
5. Milter does not receive some "macros" provided by sendmail.

Restrictions 1 and 2 are inherent to the maddy checks interface and cannot be
removed without major changes to it. Restrictions 3, 4 and 5 are temporary due to
incomplete implementation.

```
check.milter {
	endpoint <endpoint>
	fail_open false
}

milter <endpoint>
```

## Arguments

When defined inline, the first argument specifies endpoint to access milter
via. See below.

## Configuration directives

**Syntax:** endpoint _scheme://path_ ++
**Default:** not set

Specifies milter protocol endpoint to use.
The endpoit is specified in standard URL-like format:
'tcp://127.0.0.1:6669' or 'unix:///var/lib/milter/filter.sock'

**Syntax:** fail_open _boolean_ ++
**Default:** false

Toggles behavior on milter I/O errors. If false ("fail closed") - message is
rejected with temporary error code. If true ("fail open") - check is skipped.

## rspamd check (check.rspamd)

The 'rspamd' module implements message filtering by contacting the rspamd
server via HTTP API.

```
check.rspamd {
	tls_client { ... }
	api_path http://127.0.0.1:11333
	settings_id whatever
	tag maddy
	hostname mx.example.org
	io_error_action ignore
	error_resp_action ignore
	add_header_action quarantine
	rewrite_subj_action quarantine
	flags pass_all
}

rspamd http://127.0.0.1:11333
```

## Configuration directives

*Syntax:* tls_client { ... } ++
*Default:* not set

Configure TLS client if HTTPS is used, see *maddy-tls*(5) for details.

*Syntax:* api_path _url_ ++
*Default:* http://127.0.0.1:11333

URL of HTTP API endpoint. Supports both HTTP and HTTPS and can include
path element.

*Syntax:* settings_id _string_ ++
*Default:* not set

Settings ID to pass to the server.

*Syntax:* tag _string_ ++
*Default:* maddy

Value to send in MTA-Tag header field.

*Syntax:* hostname _string_ ++
*Default:* value of global directive

Value to send in MTA-Name header field.

*Syntax:* io_error_action _action_ ++
*Default:* ignore

Action to take in case of inability to contact the rspamd server.

*Syntax:* error_resp_action _action_ ++
*Default:* ignore

Action to take in case of 5xx or 4xx response received from the rspamd server.

*Syntax:* add_header_action _action_ ++
*Default:* quarantine

Action to take when rspamd requests to "add header".

X-Spam-Flag and X-Spam-Score are added to the header irregardless of value.

*Syntax:* rewrite_subj_action _action_ ++
*Default:* quarantine

Action to take when rspamd requests to "rewrite subject".

X-Spam-Flag and X-Spam-Score are added to the header irregardless of value.

*Syntax:* flags _string list..._ ++
*Default:* pass_all

Flags to pass to the rspamd server.
See https://rspamd.com/doc/architecture/protocol.html for details.
