# SMTP message routing (pipeline)

# Message pipeline

A message pipeline is a set of module references and associated rules that
describe how to handle messages.

The pipeline is responsible for

- Running message filters (called "checks"), (e.g. DKIM signature verification,
  DNSBL lookup, and so on).
- Running message modifiers (e.g. DKIM signature creation).
- Associating each message recipient with one or more delivery targets.
  Delivery target is a module that does the final processing (delivery) of the
  message.

Message handling flow is as follows:

- Execute checks referenced in top-level `check` blocks (if any)
- Execute modifiers referenced in top-level `modify` blocks (if any)
- If there are `source` blocks - select one that matches the message sender (as
  specified in MAIL FROM). If there are no `source` blocks - the entire
  configuration is assumed to be the `default_source` block.
- Execute checks referenced in `check` blocks inside the selected `source` block
  (if any).
- Execute modifiers referenced in `modify` blocks inside selected `source`
  block (if any).

Then, for each recipient:

- Select the `destination` block that matches it. If there are
  no `destination` blocks - the entire used `source` block is interpreted as if it
  was a `default_destination` block.
- Execute checks referenced in the `check` block inside the selected `destination`
  block (if any).
- Execute modifiers referenced in `modify` block inside the selected `destination`
  block (if any).
- If the used block contains the `reject` directive - reject the recipient with
  the specified SMTP status code.
- If the used block contains the `deliver_to` directive - pass the message to the
  specified target module. Only recipients that are handled
  by the used block are visible to the target.

Each recipient is handled only by a single `destination` block, in case of
overlapping `destination` - the first one takes priority.

```
destination example.org {
    deliver_to targetA
}
destination example.org { # ambiguous and thus not allowed
    deliver_to targetB
}
```

Same goes for `source` blocks, each message is handled only by a single block.

Each recipient block should contain at least one `deliver_to` directive or
`reject` directive. If `destination` blocks are used, then
`default_destination` block should also be used to specify behavior for
unmatched recipients.  Same goes for source blocks, `default_source` should be
used if `source` is used.

That is, pipeline configuration should explicitly specify behavior for each
possible sender/recipient combination.

Additionally, directives that specify final handling decision (`deliver_to`,
`reject`) can't be used at the same level as source/destination rules.
Consider example:

```
destination example.org {
    deliver_to local_mboxes
}
reject
```

It is not obvious whether `reject` applies to all recipients or
just for non-example.org ones, hence this is not allowed.

Complete configuration example using all of the mentioned directives:

```
check {
    # Run a check to make sure source SMTP server identification
    # is legit.
    spf
}

# Messages coming from senders at example.org will be handled in
# accordance with the following configuration block.
source example.org {
    # We are example.com, so deliver all messages with recipients
    # at example.com to our local mailboxes.
    destination example.com {
        deliver_to &local_mailboxes
    }

    # We don't do anything with recipients at different domains
    # because we are not an open relay, thus we reject them.
    default_destination {
        reject 521 5.0.0 "User not local"
    }
}

# We do our business only with example.org, so reject all
# other senders.
default_source {
    reject
}
```

## Directives


### check _block name_ { ... }
Context: pipeline configuration, source block, destination block

List of the module references for checks that should be executed on
messages handled by block where 'check' is placed in.

Note that message body checks placed in destination block are currently
ignored. Due to the way SMTP protocol is defined, they would cause message to
be rejected for all recipients which is not what you usually want when using
such configurations.

Example:

```
check {
    # Reference implicitly defined default configuration for check.
    spf

    # Inline definition of custom config.
    spf {
         # Configuration for spf goes here.
         permerr_action reject
    }
}
```

It is also possible to define the block of checks at the top level
as "checks" module and reference it using & syntax. Example:

```
checks inbound_checks {
	spf
	dkim
}

# ... somewhere else ...
{
	...
	check &inbound_checks
}
```

---

### modify { ... }
Default: not specified<br>
Context: pipeline configuration, source block, destination block

List of the module references for modifiers that should be executed on
messages handled by block where 'modify' is placed in.

Message modifiers are similar to checks with the difference in that checks
purpose is to verify whether the message is legitimate and valid per local
policy, while modifier purpose is to post-process message and its metadata
before final delivery.

For example, modifier can replace recipient address to make message delivered
to the different mailbox or it can cryptographically sign outgoing message
(e.g. using DKIM). Some modifier can perform multiple unrelated modifications
on the message.

**Note**: Modifiers that affect source address can be used only globally or on
per-source basis, they will be no-op inside destination blocks. Modifiers that
affect the message header will affect it for all recipients.

It is also possible to define the block of modifiers at the top level
as "modiifers" module and reference it using & syntax. Example:

```
modifiers local_modifiers {
	replace_rcpt file /etc/maddy/aliases
}

# ... somewhere else ...
{
	...
	modify &local_modifiers
}
```

---

### reject _smtp-code_ _smtp-enhanced-code_ _error-description_ <br>reject _smtp-code_ _smtp-enhanced-code_ <br>reject _smtp-code_ <br>reject
Context: destination block

Messages handled by the configuration block with this directive will be
rejected with the specified SMTP error.

If you aren't sure which codes to use, use 541 and 5.4.0 with your message or
just leave all arguments out, the error description will say "message is
rejected due to policy reasons" which is usually what you want to mean.

`reject` can't be used in the same block with `deliver_to` or
`destination`/`source` directives.

Example:

```
reject 541 5.4.0 "We don't like example.org, go away"
```

---

### deliver_to _target-config-block_
Context: pipeline configuration, source block, destination block

Deliver the message to the referenced delivery target. What happens next is
defined solely by used target. If `deliver_to` is used inside `destination`
block, only matching recipients will be passed to the target.

---

### source_in _table-reference_ { ... }
Context: pipeline configuration

Handle messages with envelope senders present in the specified table in
accordance with the specified configuration block.

Takes precedence over all `sender` directives.

Example:

```
source_in file /etc/maddy/banned_addrs {
	reject 550 5.7.0 "You are not welcome here"
}
source example.org {
	...
}
...
```

See `destination_in` documentation for note about table configuration.

---

### source _rules..._ { ... }
Context: pipeline configuration

Handle messages with MAIL FROM value (sender address) matching any of the rules
in accordance with the specified configuration block.

"Rule" is either a domain or a complete address. In case of overlapping
'rules', first one takes priority. Matching is case-insensitive.

Example:

```
# All messages coming from example.org domain will be delivered
# to local_mailboxes.
source example.org {
    deliver_to &local_mailboxes
}
# Messages coming from different domains will be rejected.
default_source {
    reject 521 5.0.0 "You were not invited"
}
```

---

### reroute { ... }
Context: pipeline configuration, source block, destination block

This directive allows to make message routing decisions based on the
result of modifiers. The block can contain all pipeline directives and they
will be handled the same with the exception that source and destination rules
will use the final recipient and sender values (e.g. after all modifiers are
applied).

Here is the concrete example how it can be useful:

```
destination example.org {
    modify {
        replace_rcpt file /etc/maddy/aliases
    }
    reroute {
        destination example.org {
            deliver_to &local_mailboxes
        }
        default_destination {
            deliver_to &remote_queue
        }
    }
}
```

This configuration allows to specify alias local addresses to remote ones
without being an open relay, since remote_queue can be used only if remote
address was introduced as a result of rewrite of local address.

**Warning**: If you have DMARC enabled (default), results generated by SPF
and DKIM checks inside a reroute block **will not** be considered in DMARC
evaluation.

---

### destination_in _table-reference_ { ... }
Context: pipeline configuration, source block

Handle messages with envelope recipients present in the specified table in
accordance with the specified configuration block.

Takes precedence over all 'destination' directives.

Example:

```
destination_in file /etc/maddy/remote_addrs {
	deliver_to smtp tcp://10.0.0.7:25
}
destination example.com {
	deliver_to &local_mailboxes
}
...
```

Note that due to the syntax restrictions, it is not possible to specify
extended configuration for table module. E.g. this is not valid:

```
destination_in sql_table {
	dsn ...
	driver ...
} {
	deliver_to whatever
}
```

In this case, configuration should be specified separately and be referneced
using '&' syntax:

```
table.sql_table remote_addrs {
	dsn ...
	driver ...
}

whatever {
	destination_in &remote_addrs {
		deliver_to whatever
	}
}
```

---

### destination _rule..._ { ... }
Context: pipeline configuration, source block

Handle messages with RCPT TO value (recipient address) matching any of the
rules in accordance with the specified configuration block.

"Rule" is either a domain or a complete address. Duplicate rules are not
allowed. Matching is case-insensitive.

Note that messages with multiple recipients are split into multiple messages if
they have recipients matched by multiple blocks. Each block will see the
message only with recipients matched by its rules.

Example:

```
# Messages with recipients at example.com domain will be
# delivered to local_mailboxes target.
destination example.com {
    deliver_to &local_mailboxes
}

# Messages with other recipients will be rejected.
default_destination {
    rejected 541 5.0.0 "User not local"
}
```

## Reusable pipeline snippets (msgpipeline module)

The message pipeline can be used independently of the SMTP module in other
contexts that require a delivery target via `msgpipeline` module.

Example:

```
msgpipeline local_routing {
    destination whatever.com {
        deliver_to dummy
    }
}

# ... somewhere else ...
deliver_to &local_routing
```