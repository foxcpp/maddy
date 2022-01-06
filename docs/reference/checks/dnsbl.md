# DNSBL lookup

The check.dnsbl module implements checking of source IP and hostnames against a set
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

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging.

**Syntax**: check\_early _boolean_ <br>
**Default**: no

Check BLs before mail delivery starts and silently reject blacklisted clients.

For this to work correctly, check should not be used in source/destination
pipeline block.

In particular, this means:
- No logging is done for rejected messages.
- No action is taken if quarantine\_threshold is hit, only reject\_threshold
  applies.
- defer\_sender\_reject from SMTP configuration takes no effect.
- MAIL FROM is not checked, even if specified.

If you often get hit by spam attacks, it is recommended to enable this
setting to save server resources.

**Syntax**: quarantine\_threshold _integer_ <br>
**Default**: 1

DNSBL score needed (equals-or-higher) to quarantine the message.

**Syntax**: reject\_threshold _integer_ <br>
**Default**: 9999

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

**Syntax**: client\_ipv4 _boolean_ <br>
**Default**: yes

Whether to check address of the IPv4 clients against the list.

**Syntax**: client\_ipv6 _boolean_ <br>
**Default**: yes

Whether to check address of the IPv6 clients against the list.

**Syntax**: ehlo _boolean_ <br>
**Default**: no

Whether to check hostname specified n the HELO/EHLO command
against the list.

This works correctly only with domain-based DNSBLs.

**Syntax**: mailfrom _boolean_ <br>
**Default**: no

Whether to check domain part of the MAIL FROM address against the list.

This works correctly only with domain-based DNSBLs.

**Syntax**: responses _cidr|ip..._ <br>
**Default**: 127.0.0.1/24

IP networks (in CIDR notation) or addresses to permit in list lookup results.
Addresses not matching any entry in this directives will be ignored.

**Syntax**: score _integer_ <br>
**Default**: 1

Score value to add for the message if it is listed.

If sum of list scores is equals or higher than quarantine\_threshold, the
message will be quarantined.

If sum of list scores is equals or higher than rejected\_threshold, the message
will be rejected.

It is possible to specify a negative value to make list act like a whitelist
and override results of other blocklists.
