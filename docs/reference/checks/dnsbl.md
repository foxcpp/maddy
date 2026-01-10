# DNSBL lookup

The check.dnsbl module implements checking of source IP and hostnames against a set
of DNS-based Blackhole lists (DNSBLs).

Its configuration consists of module configuration directives and a set
of blocks specifying lists to use and kind of lookups to perform on them.

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
    
    # Example with per-response-code scoring (new in 0.8)
    zen.spamhaus.org {
        client_ipv4 yes
        client_ipv6 yes
        
        # SBL - Spamhaus Block List (known spam sources)
        response 127.0.0.2 127.0.0.3 {
            score 10
            message "Listed in Spamhaus SBL. See https://check.spamhaus.org/"
        }
        
        # XBL - Exploits Block List (compromised hosts)
        response 127.0.0.4 127.0.0.5 127.0.0.6 127.0.0.7 {
            score 10
            message "Listed in Spamhaus XBL. See https://check.spamhaus.org/"
        }
        
        # PBL - Policy Block List (dynamic IPs)
        response 127.0.0.10 127.0.0.11 {
            score 5
            message "Listed in Spamhaus PBL. See https://check.spamhaus.org/"
        }
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

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### check_early _boolean_
Default: `no`

Check BLs before mail delivery starts and silently reject blacklisted clients.

For this to work correctly, check should not be used in source/destination
pipeline block.

In particular, this means:

- No logging is done for rejected messages.
- No action is taken if `quarantine_threshold` is hit, only `reject_threshold`
  applies.
- `defer_sender_reject` from SMTP configuration takes no effect.
- MAIL FROM is not checked, even if specified.

If you often get hit by spam attacks, it is recommended to enable this
setting to save server resources.

---

### quarantine_threshold _integer_
Default: `1`

DNSBL score needed (equals-or-higher) to quarantine the message.

---

### reject_threshold _integer_
Default: `9999`

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

### client_ipv4 _boolean_
Default: `yes`

Whether to check address of the IPv4 clients against the list.

---

### client_ipv6 _boolean_
Default: `yes`

Whether to check address of the IPv6 clients against the list.

---

### ehlo _boolean_
Default: `no`

Whether to check hostname specified n the HELO/EHLO command
against the list.

This works correctly only with domain-based DNSBLs.

---

### mailfrom _boolean_
Default: `no`

Whether to check domain part of the MAIL FROM address against the list.

This works correctly only with domain-based DNSBLs.

---

### responses _cidr_ | _ip..._
Default: `127.0.0.1/24`

IP networks (in CIDR notation) or addresses to permit in list lookup results.
Addresses not matching any entry in this directives will be ignored.

---

### score _integer_
Default: `1`

Score value to add for the message if it is listed.

If sum of list scores is equals or higher than `quarantine_threshold`, the
message will be quarantined.

If sum of list scores is equals or higher than `rejected_threshold`, the message
will be rejected.

It is possible to specify a negative value to make list act like a whitelist
and override results of other blocklists.

**Note:** When using `response` blocks (see below), the score from matching response
rules is used instead of this flat score value.

---

### response _ip..._

**New in 0.8**

Defines per-response-code rules for scoring and custom messages. This is useful
for combined DNSBLs like Spamhaus ZEN that return different codes for different
listing types.

Each `response` block takes one or more IP addresses or CIDR ranges as arguments
and contains the following directives:

#### score _integer_
**Required**

Score to add when this response code is returned. If multiple response codes
are returned by the DNSBL, scores are summed together.

#### message _string_
**Optional**

Custom rejection or quarantine message to include when this response code
matches. This message is shown to the client or logged when the threshold
is reached.

**Example:**

```
zen.spamhaus.org {
    client_ipv4 yes
    
    # High severity - known spam sources
    response 127.0.0.2 127.0.0.3 {
        score 10
        message "Listed in Spamhaus SBL"
    }
    
    # Lower severity - dynamic IPs
    response 127.0.0.10 127.0.0.11 {
        score 5
        message "Listed in Spamhaus PBL"
    }
}
```

**Backwards compatibility:** When `response` blocks are not used, the legacy
`responses` and `score` directives work as before.
