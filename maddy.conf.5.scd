maddy.conf(5) "maddy mail server"

# DESCRIPTION

maddy does have a configuration file format close in style and conventions to
Caddyfile format used in caddy but with different high-level semantics. This
man page documents both syntax and high-level structure of documentation and
includes a description of all usable directives.

Default location of main configuration file is /etc/maddy/maddy.conf.

Note, StateDirectory everywhere in this file refers to the value
of -state command line argument, which defaults to /var/lib/maddy.
LibexecDirectory everywhere in this file refers to the value
of -libexec command line argument, which defaults to /usr/lib/maddy.

# SYNTAX

Configuration consists of newline-delimited "directives". Each directive can
have zero or more arguments.

```
directive0
directive1 arg0 arg1
```

Any line starting with # is ignored. Empty lines are ignored too.

## Quoting

Strings with whitespace should be wrapped into double quotes to make sure they
will be interpreted as a single argument.

```
directive0 two arguments
directive1 "one argument"
```

String wrapped in quotes may contain newlines and they will not be interpreted
as a directive separator.

```
directive0 "one long big
argument for directive0"
```

Quotes and only quotes can be escaped inside literals: \\"

## Blocks

A directive may have several subdirectives. They are written in a {-enclosed
block like this:
```
directive0 arg0 arg1 {
    subdirective0 arg0 arg1
    subdirective1 etc
}
```

Subdirectives can have blocks too.

```
directive0 {
    subdirective0 {
        subdirective2 {
            a
            b
            c
        }
    }
    subdirective1 { }
}
```

Level of nesting is limited, but you should never hit the limit with correct
configuration.

In most cases, an empty block is equivalent to no block:
```
directive { }
directive2 # same as above
```

## Environment variables

Environment variables can be referenced in the configuration using either
{$VARIABLENAME} or {%VARIABLENAME%} syntax.

Non-existent variables are expanded to empty strings and not removed from
the arguments list.  In the following example, directive0 will have one argument
independently of whether $VAR is defined.

```
directive0 {$VAR}
```

## Snippets & imports

You can reuse blocks of configuration by defining them as "snippets". Snippet
is just a directive with a block, declared tp top level (not inside any blocks)
and with a directive name wrapped in curly braces.

```
(snippetname) {
    a
    b
    c
}
```

The snippet can then be referenced using 'import' meta-directive.

```
unrelated0
unrelated1
import snippetname
```

The above example will be expanded into the following configuration:

```
unrelated0
unrelated1
a
b
c
```

Import statement also can be used to include content from other files. It works
exactly the same way as with snippets but the file path should be used instead.
The path can be either relative to the location of the currently processed
configuration file or absolute. If there are both snippet and file with the
same name - snippet will be used.

```
# /etc/maddy/tls.conf
tls long_path_to_certificate long_path_to_private_key

# /etc/maddy/maddy.conf
smtp smtp://0.0.0.0 {
    import tls.conf
}
```

```
# Expanded into:
smtp smtp://0.0.0.0 {
    tls long_path_to_certificate long_path_to_private_key
}
```

The imported file can introduce new snippets and they can be referenced in any
processed configuration file.

## Duration values

Directives that accept duration use the following format: A sequence of decimal digits with an optional fraction and 
unit suffix (zero can be specified without a suffix). If multiple values are specified, they
will be added.

Valid unit suffixes: "h" (hours), "m" (minutes), "s" (seconds), "ms" (milliseconds).
Implementation also accepts us and ns for microseconds and nanoseconds, but these 
values are useless in practice.

Examples:
```
1h
1h 5m 
1h5m
0
```

## Data size values

Similar to duration values, but fractions are not allowed and suffixes are different. 

Valid unit suffixes: "G" (gibibyte, 1024^3 bytes), "M" (mebibyte, 1024^2 bytes), 
"K" (kibibyte, 1024 bytes), "B" or "b" (byte).

Examples:
```
32M
3M 5K
5b
```

Also note that the following is not valid, unlike Duration values syntax:
```
32M5K
```

# MODULAR FRAMEWORK

maddy architecture consists of several components called "modules". Each module
does have its separate configuration block. The module can be used multiple times
with different configurations by giving each configuration block unique name.

If a module needs to use functionality of the other module, it references
the used configuration block in its configuration. Here is the example:
```
module1 config1 {
    blah_blah
}

module2 config2 {
    # This directive means that the module2 with configuration
    # block config2 will module1 for some of its functionality. For
    # example, it may send messages to it for some handling (filtering
    # or delivery, you got the idea). module1 will handle them in accordance
    # with the config1 block.
    target config1
}
```


Generic syntax for module configuration block is as follows:

```
module_name config_block_name optional_aliases... {
    configuration
    directives
    for_this
    module
}
```
If you specify more than one config_block_name, they all will be usable.
Basically, they will be aliased to the first name.

If config_block_name is omitted, it will be the same as module_name.
Configuration block name must be unique across all configuration.

Remember that
```
    modname { }
```
is the same as
```
    modname
```

Hence
```
    modname modname {}
```
is same as just
```
    modname
```

Remaining man page sections describe various modules you can use in your
configuration.

## "Inline" configuration blocks

In most cases where you are supposed to specify configuration block name, you
can instead write module name and include configuration block itself.

Like that:
```
something {
    auth sql {
        driver sqlite3
        dsn auth.db
    }
}
```
instead of
```
sql thing_name {
    driver sqlite3
    dsn auth.db
}

something {
    auth thing_name
}
```

In that syntax, arguments specified after module name are not considered to be
configuration block names and instead passed directly to the module.
Some modules use them to allow writing more compact configuration, e.g. sql module
allows you to specify driver and DSN in the inline arguments this way:
```
something {
    auth sql sqlite3 auth.db
} 
```


# GLOBAL DIRECTIVES

These directives can be used outside of any block.

## hostname <domain>

Specify the local hostname to use. This is the unique name for the mail server.
In most cases you want to specify a domain that resolves to the server IP here.

## autogenerated_msg_domain <domain>

Domain that is used in From field for auto-generated messages (such as Delivery
Status Notifications).

## tls <cert_file> <pkey_file>

Default TLS certificate to use for all endpoints. Can be "tls off" instead to
disable TLS fully (not recommended!).

Must be present in either all endpoint modules configuration blocks or as
global directive.

You can also specify other configuration options such as cipher suites and TLS
version. See TLS CONFIGURATION for details.

## log <targets...>

Write log to one of more "targets".
The target can be one or the following:
- stderr
Write logs to stderr, this is the default.
- stderr_ts
Write logs to stderr with timestamps.
- syslog
Send logs to the local syslog daemon.
- off
Do nothing. Used to disable logging fully: log off
It Can't be combined with other targets.
- file path
Write (append) logs to file.

For example:
```
log syslog /var/log/maddy.log
```

*Note:* Maddy does not perform log files rotation, this is the job of the
logrotate daemon.

## debug [yes/no]

Enable verbose logging. You don't need that unless you are reporting a bug.

## storage_perdomain [yes/no]

Don't remove domain part when accessing the underlying storage and require it
to be present. Can be used if you want user@domain1 and user@domain2 to be
different accounts at the storage level.

## auth_perdomain [yes/no]

If yes - authentication requests that specify just username without the domain
will be rejected as invalid and domain part will be used as part of login name.

## auth_domains <domain0> [domain1] ...

Domains that should be allowed in username.

For example, if auth_domains is set to "domain1 domain2", then
username, username@domain1 and username@domain2 will be accepted as valid login
name in addition to just username.

If used without auth_perdomain, domain part will be removed from login before
check with underlying auth. mechanism. If auth_perdomain is set, then
auth_domains must be also set and domain part WILL NOT be removed before check.

# TLS CONFIGURATION

You can specify other TLS-related options in a configuration block:

```
tls cert.pem cert.pem {
    protocols tls1.3
}
```

Valid configuration directives:

## protocols <min> <max>
## protcools <value>

Minimum/maximum accepted TLS version. If only one value is specified, it will
be the only one usable version.

Valid values are: tls1.0, tls1.1, tls1.2, tls1.3

Default values are defined by crypto/tls, at the time of writting they were TLS
1.0 and TLS 1.2.

## ciphers <cipher0> [cipher1] [cipher2] ...

List of supported cipher suites, in preference order.
Default is defined by crypto/tls and may be different depending on
CPU architecture and Go version. Ignored if TLS 1.3 is used.

Valid values:
- RSA-WITH-RC4128-SHA
- RSA-WITH-3DES-EDE-CBC-SHA
- RSA-WITH-AES128-CBC-SHA
- RSA-WITH-AES256-CBC-SHA
- RSA-WITH-AES128-CBC-SHA256
- RSA-WITH-AES128-GCM-SHA256
- RSA-WITH-AES256-GCM-SHA384
- ECDHE-ECDSA-WITH-RC4128-SHA
- ECDHE-ECDSA-WITH-AES128-CBC-SHA
- ECDHE-ECDSA-WITH-AES256-CBC-SHA
- ECDHE-RSA-WITH-RC4128-SHA
- ECDHE-RSA-WITH-3DES-EDE-CBC-SHA
- ECDHE-RSA-WITH-AES128-CBC-SHA
- ECDHE-RSA-WITH-AES256-CBC-SHA
- ECDHE-ECDSA-WITH-AES128-CBC-SHA256
- ECDHE-RSA-WITH-AES128-CBC-SHA256
- ECDHE-RSA-WITH-AES128-GCM-SHA256
- ECDHE-ECDSA-WITH-AES128-GCM-SHA256
- ECDHE-RSA-WITH-AES256-GCM-SHA384
- ECDHE-ECDSA-WITH-AES256-GCM-SHA384
- ECDHE-RSA-WITH-CHACHA20-POLY1305
- ECDHE-ECDSA-WITH-CHACHA20-POLY1305

## curve <curve0> [curve1] ...

The elliptic curves that will be used in an ECDHE handshake, in preference
order.

Valid values: p256, p384, p521, X25519.

# ADDRESS DEFINITIONS

Maddy configuration uses URL-like syntax to specify all network addresses and
their protocols.

## lmtp+unix://file_path

Describes local Unix socket using the LMTP protocol for communication.

```
lmtp+unix:///var/maddy/local_submission.sock
```

## smtp://ADDRESS[:PORT] smtps://ADDRESS[:PORT]

Describes TCP/IP socket using SMTP protocol for communication.
smtp:// uses Implicit TLS.

If a port is not specified - it is 25 for smtp:// and 465 for smtps://.

```
smtp://0.0.0.0:587
smtps://127.0.0.1:465
smtp://0.0.0.0
```

## imap://ADDRESS[:PORT] imaps://ADDRESS[:PORT]

Describes TCP/IP socket using the IMAP protocol for communication.
imaps:// uses Implicit TLS.

If a port is not specified - it is 143 for imap:// and 993 for imaps://.

```
imap://0.0.0.0:993
imaps://127.0.0.1:143
imap://0.0.0.0
```

# DISPATCHING DIRECTIVES

maddy have a generic framework for message filtering and simple
per-sender/per-recipient routing dispatcher.

Below are directives you can use in the configuration of the modules that
act as a message sources (e.g. a SMTP endpoint).

## check { ... }

List of the configuration block names for checks that should
be executed on all incoming messages. In most cases, checks have
implicitly defined top-level configuration block, so you don't have
to declare your own.

The way check failure is handled is defined by the check itself, 
usually message will be quarantined.

See CHECKS MODULES for what you can use here.

Note that message body checks placed in destination block are currently
ignored. Due to the way SMTP protocol is defined, they would cause message to
be rejected for all recipients which is not what you usually want when using
such configurations.

Example:
```
check {
    # Reference implicitly defined default configuration.
    check_source_hostname

    # Inline definition.
    check_source_mx {
         # Configuration for check_source_mx goes here.
         debug
    }
}
```

## modify { ... }

List of the configuration block names for modifier that should
be executed on all incoming messages. In most cases, modifier have implicitly defined 
top-level configuration block, so you don't have to declare your own.

Message modifiers are similar to checks with the difference in that
checks purpose is to verify whether the message is legitimate and valid
per local policy, while modifier purpose is to post-process message
and its metadata before final delivery.

For example, modifier can replace recipient address to make message
delivered to the different mailbox or it can cryptographically sign
outgoing message (e.g. using DKIM protocol). Some modifier can perform
multiple modifications on the message.

See MODIFY MODULES for what you can use here.

NOTE: Modifiers that affect the message header or source address can
be used only globally or on per-source basis, they will be no-op
inside destination blocks.

## reject [smtp code] [smtp enhanced code] [error description]

Messages handled by the configuration block with this directive will be rejected
with the specified SMTP error.

If you aren't sure which codes to use, use 541 and 5.4.0 with your message or just
leave all arguments out, the error description will say "message is rejected
due to policy reasons" which is usually what you want to mean.

'reject' can't be used in the same block with 'deliver_to' or 'destination/source' directives.

Example:
```
reject 541 5.4.0 "We don't like example.org, go away"
```

## source <rules...> { ... }

Handle messages with MAIL FROM value (sender address) matching any of the rules in
accordance with the specified configuration block.

"Rule" is either a domain or a complete address. Duplicate rules are not allowed.

A source block can contain the following directives: 'destination'/'default_destination',
'check', 'modify', 'deliver_to`, 'reject'. If you use the 'source' directive, then you 
must also use the 'default_source' directive to specify how to handle messages 
with senders not matched by any rules.

A 'source' block can't be used with other directives at the same level.

Example:
```
# All messages coming from example.org domain will be delivered
# to local_mailboxes.
source example.org {
    deliver_to local_mailboxes
}
# Messages coming from different domains will be rejected.
default_source {
    reject 521 5.0.0 "Go away"
}

# You can't put the following directives at the same level as blocks above:
# reject
# deliver_to whatever
# destination ...
# default_destination
```

## destination <rule...> { ... }

Handle messages with RCPT TO value (recipient address) matching any of the rules in
accordance with the specified configuration block.

"Rule" is either a domain or a complete address. Duplicate rules are not allowed.

Note that messages with multiple recipients are split into multiple messages if
they have recipients matched by multiple blocks. Each block will see the message
only with recipients matched by its rules.

A destination block can contain the following directives: 'deliver_to', 'reject','check',
'modify'. If you use the 'destination' directive then you must also use the
'default_destination' directive to specify how to handle messages with recipients 
not matched by any rules.

A 'destination' block can't be used with other directives at the same level.

Example:
```
# Messages with recipients at example.com domain will be
# delivered to local_mailboxes target.
destination example.com {
    deliver_to local_mailboxes
}

# Messages with other recipients will be rejected.
default_destination {
    rejected 541 5.0.0 "User not local"
}
```



Complete configuration example using all of the mentioned directives:
```
check {
    # Run a check to make sure source SMTP server identification
    # is legit.
    check_source_hostname
}

# Messages coming from senders at example.org will be handled in
# accordance with the following configuration block.
source example.org {
    # We are example.com, so deliver all messages with recipients
    # at example.com to our local mailboxes.
    destination example.com {
        deliver_to local_mailboxes
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

# IMAP MODULE

IMAP4rev1 server endpoint. Endpoints are special-case modules that use config
directive arguments to specify listening addresses.

```
imap imap://0.0.0.0 imaps://0.0.0.0:993 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    storage spool
}
```

## tls ...

Override global tls directive.

## auth <block_name>

Use an authentication module with a specified configuration block name. *Required.*

## storage <block_name>

Use a storage module with a specified configuration block name. *Required.*

## insecure_auth [yes/no]

Allow plain-text authentication over unencrypted connections. Not recommended!
Enabled automatically if TLS is disabled.

## io_debug [yes/no]

Write all commands and responses to stderr.

## debug [yes/no]

Enable verbose logging only for this configuration block.

# SMTP MODULE

ESMTP server endpoint. Endpoints are special-case modules that use config
directive arguments to specify listening addresses.

See DISPATCHING DIRECTIVES for the description of the
conditional message dispatching and filtering configuration.

```
smtp smtp://0.0.0.0:25 smtps://0.0.0.0:587 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    hostname emersion.fr

    ...
}
```

## hostname ...

Override the global hostname directive. The hostname is used in EHLO/HELO greetings.

## tls ...

Override global tls directive.

## io_debug [yes/no]

Write all commands and responses to stderr.

## debug [yes/no]

Enable verbose logging only for this configuration block.

## insecure_auth

Allow plain-text authentication over unencrypted connections. Not recommended!
Enabled automatically if TLS is disabled.

## read_timeout <duration>`

I/O read timeout. The default is 10 minutes.

See SYNTAX for value format (unit suffixes, etc).

## write_timeout <duration>

I/O write timeout. The default is 1 minute.

See SYNTAX for value format (unit suffixes, etc).

## max_message_size <size>

Limit the size of incoming messages to 'size'. The default is 32 MiB.

See SYNTAX for value format (unit suffixes, etc).

## auth <block_name>

Use an authentication module with a specified configuration block name.
*Required.*

## submission [yes/no]

Preprocess messages before pushing them to target and require authentication
for all operations. You should use it for Submission protocol endpoints.

## defer_sender_reject [yes/no]

Run sender-based checks and dispatching logic when first RCPT TO command
is received. This allows maddy to log recipient address of the rejected
message and also improves interoperability with (improperly implemented) 
clients that don't expect an error early in session.

Enabled by default. Disable with 'defer_sender_reject no'.

## target <block_name>

Deliver messages to the delivery target defined by a specified configuration
block name.

# SUBMISSION MODULE

Alias to smtp module with submission directive used by default.

# SQL MODULE

SQL-based storage backend. Can be used as a storage backend (for IMAP),
authentication provider (IMAP & SMTP) or delivery target (SMTP).

See https://github.com/foxcpp/go-imap-sql for more information.

Valid configuration directives:

## driver <string>

REQUIRED.

Use a specified driver to communicate with the database.  Supported values:
sqlite3, postgres.

## dsn <string>

REQUIRED.

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.
For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

## fsstore
## fsstore <directory>

REQUIRED.

Store message contents in filesystem directory.

If directory path is not specified - it will be derived from configuration
block name ('StateDirectory/sql-NAME-fsstore').

## compression <algorithm> [params]

Apply compression to message contents.

Supported algorithms are: lz4, zstd.
'params' specify compression level.

Default is 'compress off' which disables compression.

## appendlimit <size>

Don't allow users to add new messages larger than 'size'. The default is 32 MiB.

See SYNTAX for size value format (unit suffixes, etc).

## debug [yes/no]

Enable verbose logging only for this configuration block.

## junk_mailbox <name>

Default directory to put quarantined messages in. Note that this setting is not used if user 
does have a mailbox with "Junk" special-use attribute. 

## sqlite_exclusive_lock
## sqlite_cache_size <value>
## sqlite_busy_timeout <value>

SQLite3-specific performance tuning options.
cache_size and busy_timeout change corresponding PRAGMA options. exclusive_lock
sets locking_mode PRAGMA to EXCLUSIVE.

If cache_size is not used, SQLite3 default is used. If busy_timeout is not set,
500000 is used.

See https://sqlite.org/pragma.html for more details.

## storage_perdomain [yes/no]

Override global storage_perdomain directive.

## auth_perdomain [yes/no]
## auth_domains ...

Override corresponding global directives.

# QUEUE MODULE

Queue module buffers messages on disk and retries delivery multiple times to
ensure reliable delivery.

```
queue {
    max_parallelism 16
    max_tries 8
    target dummy
}
```

## Inline arguments

When used inline, first argument specifies directory to use for storage.
Relative paths are relative to the StateDirectory. 


Valid configuration directives:

## target <config block>

Delivery target to use for delivery. You probably want to use 'remote' here.

## location <directory>

File system directory to use to store queued messages.

Default is StateDirectory/<configuration_block_name>.

## max_parallelism <count>

Start up to 'count' goroutines for message processing. Basically, this option
limits amount of messages tried to be delivered concurrently.

Default is 16.

## max_tries <count>

Attempt delivery up to 'count' times. Note that no more attempts will be done
is permanent error occured during previous attempt.

Default is 4.

## bounce { ... }

The block specified with the bounce directive specifies how DSN (Delivery Status Notification)
messages should be routed. The syntax of directives inside this block is described in
DISPATCHING DIRECTIVES section.

If this is block is not present in configuration, DSNs will not be generated. Note, however,
this is not what you want most of the time.

## autogenerated_msg_domain <domain>

Overrides the global autogenerated_msg_domain. Note that it must be non-empty (or specified
globally), if bounce directive is used too.

## debug [yes/no]

Enable verbose logging only for this configuration block.

# REMOTE MODULE

Module that implements message delivery to remote MTAs.
You probably want to use it with queue module for reliability.

```
remote
```

If a message check marks message as 'quarantined', remote module
will refuse to deliver message.

Valid configuration directives:

## hostname <hostname>

Override global hostname directive.

## require_tls

Always refuse to send messages over unencrypted connections.

## debug [yes/no]

Enable verbose logging only for this configuration block.

## mtasts_cache <directory>

Set location of the MTA-STS policies cache directory.
Default is StateDirectory/mtasts-cache

# EXTAUTH MODULE

Module for authentication using external helper binary. It looks for binary
named maddy-auth-helper in $PATH and libexecdir and uses it for authentication.

The protocol is very simple:
Program is launched for each authentication. Username and password are written
to stdin, adding \\n to the end. If binary exits with 0 status code -
authentication is considered successful. If the status code is 1 - authentication
is failed. If the status code is 2 - another unrelated error has happened. Additional
information should be written to stderr.

```
extauth {
    helper /usr/bin/ldap-helper
}
```

Valid configuration directives:

## helper <file_path>

Location of the helper binary. Default is LibexecDirectory/maddy-auth-helper

## debug [yes/no]

Verbose log only for this configuration block.

## auth_perdomain [yes/no]
## auth_domains ...

Override corresponding global directives.


# PAM MODULE

Implements authentication using libpam. Alternatively it can be configured to use
helper binary like extauth does. 

maddy should be built with CGo support enabled (default if C compiler is available) 
to use this module without 'use_helper' directive. 

```
smtp smtp://0.0.0.0 {
    auth pam { }
}
```

## debug [yes/no]

Verbose log only for this configuration block.

## use_helper

Use LibexecDirectory/maddy-pam-helper instead of directly calling libpam.
You need to use that if:
1. maddy is not compiled with CGo support, but maddy-pam-helper is built separately.
2. maddy is running as an unprivileged user and used PAM configuration requires additional 
privileges (e.g. when using system accounts).

For 2, you need to make maddy-pam-helper binary setuid, see 
cmd/maddy-pam-helper/README.md in source tree for details. 

TL;DR (assuming you have maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

# SHADOW MODULE

Implements authentication by reading /etc/shadow. Alternatively it can be 
configured to use helper binary like extauth does. 

```
smtp smtp://0.0.0.0 {
	auth shadow { }
}
```

## debug [yes/no]

Verbose log only for this configuration block.

## use_helper

Use LibexecDirectory/maddy-shadow-helper instead of directly reading /etc/shadow.
You need to use that if maddy is running as an unprivileged user 
privileges (e.g. when using system accounts).

You need to make maddy-shadow-helper binary setuid, see 
cmd/maddy-shadow-helper/README.md in source tree for details. 

TL;DR (assuming you have maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-shadow-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-shadow-helper
```

# CHECK MODULES

All modules in this section run one check on message and quarantine 
(or reject, for some checks) the message if it fails.

They all implement check interface and have configuration blocks defined
implicitly so you use then without an explicit block definition.

All modules in this list support the following directives:
- debug
  Enable verbose logging for this module.
- fail_action ignore
  Ignore the check failure, useful for testing deployments to see
  how useful it is.
- fail_action reject
  Reject the message instead of quarantining it.
- fail_action quarantine
  Quarantine the message, this is the default for most checks.
- fail_action score <number>
  Increase checks score by <number> (can be negative to decrease it).
- ok_score <number>
  Increase checks score by <number> if check passed (can be negative to decrease it).

## require_matching_ehlo

Check that source server hostname (from EHLO/HELO command) resolves to source
server IP.

## require_mx_record

Check that domain in MAIL FROM command does have a MX record.

## require_matching_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

# VERIFY_DKIM MODULE

This is the check module that performs verification of the DKIM signatures
present on the incoming messages.

It got an implicit configuration block defined like CHECK MODULES above.


```
check {
    ...
    verify_dkim
}
```

Valid directives:

## debug [yes/no]

Toggle debug logging only for this module.

## no_sig_action ...

Action to take when message without any signature is received.
Default is 'ignore', as recommended by RFC 6376.

Valid arguments are same as for CHECK MODULES above.

## broken_sig_action ...

Action to take when message with invalid signature is received.
Default is 'ignore', as recommended by RFC 6376.

Valid arguments are same as for CHECK MODULES above.

# REPLACE_SENDER, REPLACE_RCPT

These are modules that simply replace matching address value(s) with another 
in either MAIL FROM or RCPT TO. 

Matching is done either by full address string or regexp that should match
entire address (it is implicitly wrapped with ^ and $). In either case, 
matching is case-insensitive. 

Configuration is done using inline arguments or 'from' and 'to'
directives. See below for examples.

```
modify {
    # Replace addr@example.com with addr@example.org in MAIL FROM (message sender).
    replace_sender addr@example.com addr@example.org

    # Replace addr@example.com with addr@example.org in RCPT TO (message recipient).
    replace_rcpt addr@example.com addr@example.org

    # Examples below use replace_sender but work exactly the same way for replace_rcpt.

    # Replace any address matching /-enclosed regexp with com@example.org.
    replace_sender /(.+)@example.com/ com@example.org

    # You can also reference capture groups in the second argument.  
    replace_sender /(.+)@example.com/ $1@example.org
}
```


# DUMMY MODULE

No-op module. It doesn't need to be configured explicitly and can be referenced
using "dummy" name. It can act as a delivery target or auth.
provider. In the latter case, it will accept any credentials, allowing any
client to authenticate using any username and password (use with care!).

# SEE ALSO

*maddy*(1)
