maddy.conf(5) "maddy mail server"

# DESCRIPTION

maddy does have a configuration file format close in style and conventions to
Caddyfile format used in caddy but with different high-level semantics. This
man page documents both syntax and high-level structure of documentation and
includes a description of all usable directives.

Default location of main configuration file is /etc/maddy/maddy.conf.

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

# MODULAR FRAMEWORK

maddy architecture consists of several components called "modules". Each module
does have its separate configuration block. The module can be used multiple times
with different configurations by giving each configuration block unique name.

Generic syntax for module configuration block is as follows:

```
module_name config_block_name {
    configuration
    directives
    for_this
    module
}
```

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

Exceptions to this rule are explicitly noted in the documentation.

*Note* that in certain cases you also have to specify a name for "inline"
configuration block. This is required when the used module uses configuration
block name as a key to store persistent data.
```
smtp ... {
    deliver queue block_name_here {
        target remote
    }
}
```


# GLOBAL DIRECTIVES

These directives can be used outside of any block.

## hostname <domain>

Specify the local hostname to use. Relevant for SMTP endpoints and queues.

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

## statedir <directory>

Change directory used for all state-related files.
Default is $MADDYSTATE environment variable or `/var/lib/maddy` if $MADDYSTATE
is not set. The default value can be changed using -X linker flag during
compilation:

```
go build --ldflags '-X github.com/emersion/maddy.defaultStateDirectory=/opt/maddy/state'
```

## libexecdir <directory>

Change the directory where all auxiliary binaries are stored.
Default is $MADDYLIBEXEC environment variable or `/usr/libexec/maddy` if
$MADDYLIBEXEC is not set. The default value can be changed using -X linker
flag during compilation:

```
go build --ldflags '-X github.com/emersion/maddy.defaultLibexecDirectory=/opt/maddy/bin'
```

## debug [yes/no]

Enable verbose logging. You don't need that unless you are reporting a bug.

# TLS CONFIGURATION

You can specify other TLS-related options in a configuration block:

```
tls cert.pem cert.pem {
    protocols tls1.3
}
```

Certificate to use for client connection is picked based on SNI. If no SNI is
sent by client - first certificate is used.

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

```
smtp smtp://0.0.0.0:25 smtps://0.0.0.0:587 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    hostname emersion.fr

    destination emersion.fr {
        filter dkim verify
        deliver local
    }

    require_auth
    filter dkim sign
    deliver out-queue
}
```

Directives not mentioned below are parsed as SMTP pipeline definition (see the
corresponding section below)

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

## read_timeout <value>`

I/O read timeout in seconds. The default is 10 minutes.

## write_timeout <value>

I/O write timeout in seconds. The default is 1 minute.

## max_message_size <value>

Limit the size of incoming messages to value bytes. The default is 32 MiB.

## auth <block_name>

Use an authentication module with a specified configuration block name.
*Required.*

## submission [yes/no]

Preprocess messages before pushing them to pipeline and require authentication
for all operations. You should use it for Submission protocol endpoints.

# SMTP PIPELINE

SMTP module does have a flexible mechanism that allows you to define a custom
sequence of actions to apply on each incoming message.

Pipeline steps are applied strictly in the order they are specified in the
configuration. Steps can change the message body or recipient list. They also
can reject the message, in this case no more steps will be executed and error
will be returned to the message source (SMTP client).

You can add any number of steps you want using the following directives:

## filter <instnace_name>

Apply a "filter" to a message, instance_name is the configuration block name.
You can pass additional parameters to filter by adding key=value pairs to the
end directive, you can omit the value and just specify key if it is
supported by the filter.

## deliver <instance_name>

Same as the filter directive, but also executes certain pre-delivery
operations required by RFC 5321 (SMTP), i.e. it adds Received header to
message.

## stop
## stop <status code> <message>

Stops processing. If status code and message are specified - they will be
returned. Otherwise "250 OK: queued" will be sent.

## require_auth

Stop processing with "access denied" error if the client is not authenticated
non-anonymously.

## match [no] <field> <subtring>  { ... }
## match [no] <field> /<regexp>/  { ... }

Executes all nested steps if the condition specified in the directive is true
(field contains the specified substring).

*Note:* 'match' is low-level dispatching primitive, in most cases you should
use 'destination' instead.

If the substring is wrapped in forward slashes - it will be interpreted as a
Perl-compatible regular expression that should match field contents.

Valid "fields":
- rcpt
  Message recipient addresses, the condition is true if at least one
  recipient matches.
- rcpt_domain
  Message recipient domain, the condition is true if at least one
  recipient matches.
- from
  Message sender address.
- from_domain
  Domain part of message sender address.
- src_addr
  IP of the client who submitted the message.
- src_hostname
  Hostname reported by the client in the EHLO/HELO command.

## destination <recipient...> { ... }

For all recipients that match at least one rule - execute subblock and stop
processing, for all others - skip block and continue.

"Rule" can be either domain name, full address (should include `@`) or
a regular expression that should match full address.

Example: Deliver to "local" all messages for mailboxes on example.org and all other - to "dummy".
```
destination example.org { deliver local }
deliver dummy
```

*Note:* Don't forget that order of pipeline steps matters.
```
deliver sql
destination postmaster@example.org { deliver local }
```
In this case, messages for postmaster@example.org will be delivered to
*both* 'sql' and 'local' storage.

# SUBMISSION MODULE

Alias to smtp module with submission directive used by default.

## SQL MODULE

SQL-based storage backend. Can be used as a storage backend (for IMAP),
authentication provider (IMAP & SMTP) or delivery target (SMTP).

See https://github.com/foxcpp/go-imap-sql for more information.

Valid configuration directives:

## driver <string>

Use a specified driver to communicate with the database.  Supported values:
sqlite3, mysql, postgres.

## dsn <string>

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.
For MySQL: https://github.com/go-sql-driver/mysql#dsn-data-source-name
For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

## appendlimit <value>

Refuse to accept messages larger than `value` bytes. The default is 32 MiB.

## debug [yes/no]

Enable verbose logging only for this configuration block.

## fsstore
## fsstore <directory>

Store messages in file system directory instead of database rows. You generally
want to use this for server-based RDBMS.

If directory path is not specified - it will be derived from configuration
block name ('$MADDYSTATE/sql-NAME-fsstore').

## sqlite_exclusive_lock
## sqlite_cache_size <value>
## sqlite_busy_timeout <value>

SQLite3-specific performance tuning options.
cache_size and busy_timeout change corresponding PRAGMA options. exclusive_lock
sets locking_mode PRAGMA to EXCLUSIVE.

If cache_size is not used, SQLite3 default is used. If busy_timeout is not set,
500000 is used.

See https://sqlite.org/pragma.html for more details.

# QUEUE MODULE

Queue module buffers messages on disk and retries delivery multiple times to
ensure reliable delivery.

```
queue {
    workers 16
    max_tries 8
    target dummy
}
```

Valid configuration directives:

## location

File system directory to use to store queued messages.

Default is $MADDYSTATE/<configuration_block_name>.

## workers <count>

Start up to 'count' goroutines for message processing. Basically, this option
limits amount of messages tried to be delivered concurrently.

Default is 16.

## max_tries <count>

Attempt delivery up to 'count' times. Note that no more attempts will be done
is permanent error occured during previous attempt.

Default is 4.

## debug [yes/no]

Enable verbose logging only for this configuration block.

# REMOTE MODULE

Module that implements message delivery to remote MTAs.
You probably want to use it with queue module for reliability.

```
remote
```

Valid configuration directives:

## hostname <hostname>

Override global hostname directive.

## require_tls

Always refuse to send messages over unencrypted connections.

## debug [yes/no]

Enable verbose logging only for this configuration block.

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

Location of the helper binary.

## debug [yes/no]

Verbose log only for this configuration block.

# PAM MODULE

Same as 'extauth' module but looks for maddy-pam-helper binary by default
which implements authentication using the PAM stack.

```
pam
smtp smtp://0.0.0.0 {
    auth pam
}
```

# SHADOW MODULE

Same as 'extauth' module but looks for maddy-shadow-helper binary by default
which implements authentication using local shadow database (/etc/shadow).

```
shadow
smtp smtp://0.0.0.0 {
	auth shadow
}
```

# CHECK_* MODULES

All modules in this section run one check on message and either cause it to be
rejected.

They all implement filter interface and have configuration blocks defined
implicitly so you use them like that:
```
filter check_source_hostname
```

## check_source_hostname

Check that source server hostname (from EHLO/HELO command) resolves to source
server IP.

## check_source_mx

Check that domain in MAIL FROM command does have a MX record pointing to
source server.

## check_source_rdns

Check that source server IP does have a PTR record point to the domain
specified in EHLO/HELO command.

# DUMMY MODULE

No-op module. It doesn't need to be configured explicitly and can be referenced
using "dummy" name. It can act as a filter, delivery target, and auth.
provider. In the latter case, it will accept any credentials, allowing any
client to authenticate using any username and password (use with care!).

# SEE ALSO

*maddy*(1)
