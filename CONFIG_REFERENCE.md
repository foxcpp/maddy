### 'imap' module

IMAP4rev1 server endpoint.
The instance name is used as a listening address.

The listening address is in the URL-like form, `scheme://IP:PORT`. The scheme
must be either `imap` or `imaps`. If a port is not specified it will be derived
from the scheme, 993 for `imaps`, `143` for `imap`. If the scheme is `imaps` -
TLS is used from the start, otherwise, STARTTLS extension is enabled (unless
you disable TLS fully).

Valid configuration directives and their forms:
`<>`-enclosed values are placeholders for the actual values you need to provide.
`[]`-enclosed ones are optional.

* `tls <certificate_file> <private_key_file>` 
  Set TLS certificate & key to use.

* `tls off`
  Disable TLS (not recommended).

* `tls self_signed`
  Generate a self-signed certificate on startup. Useful only for testing.

* `auth <instance_name>` 
  Use the specified authentication provider module instead of default-auth or
  default. `instance_name` is the name of the corresponding configuration
  block.

* `storage <instance_name>`
  Use the specified storage backend module instead of default-storage or
  default. `instance_name` is the name of the corresponding configuration
  block.

* `insecureauth`
  Allow plaintext authentication over unprotected (unencrypted) connections.
  Use only for testing!

* `iodebug`
  Write all protocol commands from clients and responses to stderr.

* `errors stderr`
  Write protocol errors log to stderr (default).

* `errors stdout`
  Write protocol errors log to stdout.

* `errors <file>` 
   Write the protocol error log to the specified file (deprecated).

```
imap imap://0.0.0.0 imaps://0.0.0.0:993 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    insecureauth
    errors /var/lob/imap-errs.log
    storage spool
}
```

### 'smtp' module 

ESMTP server endpoint.
The instance name is used as a listening address.

The listening address is in the URL-like form, `scheme://IP:PORT`. The scheme
must be either `smtp` or `smtps`. If a port is not specified it will be derived
from the scheme, 465 for `smtps`, `25` for `smtp`. If the scheme is `smtps` -
TLS is used from the start, otherwise, STARTTLS extension is enabled (unless
you disable TLS fully). You can specify multiple space-separated listening
addresses (see the end of the section for example).

Valid configuration directives and their forms:
`<>`-enclosed values are placeholders for the actual values you need to provide.
`[]`-enclosed ones are optional.

* `tls <certificate_file> <private_key_file>`
  Set TLS certificate & key to use.

* `tls off`
  Disable TLS (not recommended).

* `tls self_signed`
   Generate a self-signed certificate on startup. Useful only for testing.

* `auth <instance_name>`
  Use the specified authentication provider module instead of default-auth or
  default. `instance_name` is the name of the corresponding configuration
  block.

* `insecureauth`
  Allow plaintext authentication over unprotected (unencrypted)
  connections. Use only for testing!

* `iodebug`
  Write all protocol commands from clients and responses to stderr.

* `hostname <domain>`
  Set server domain name to advertise in EHLO/HELO response and for matching
  during delivery. Required.

```
smtp smtp://0.0.0.0:25 smtps://0.0.0.0:587 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    hostname emersion.fr
}
```

##### SMTP pipeline

SMTP module does have a flexible mechanism that allows you to define a custom
sequence of actions to apply on each incoming message.

By default, it just passes emails with recipients with domain same as the
specified hostname to default-delivery or default delivery target (usually IMAP
mailbox). If the message does have non-local recipients it will be passed to
message queue for outgoing transfer.

Here are configuration directives doing the same (almost):
```
deliver default local-only
deliver out-queue remote-only
```

You can add any number of steps you want using following directives (note that
if you specify any of them default steps will not be used so you need to 
specify them explicitly!)

* `filter <instnace_name> [opts]` 
  Apply a "filter" to a message, `instance_name` is the configuration set name.
  You can pass additional parameters to filter by adding key=value pairs to the
  end directive, you can omit the value and just specify key if it is
  supported.

  A filter can mark the message as rejected and it will be dropped immediately
  (no further pipeline steps will be run).

* `deliver <instance_name> [opts]`
  Very close to the `filter` directive but the target module is not allowed to
  modify the message or associated context values. Exists purely to make
  configuration easier to understand.

* `stop` 
  Stops processing.

* `require-auth`
  Stop processing with "access denied" error if the client is not authenticated
  non-anonymously.

* `match [no] <field> <subtring>  { ... }`
  `match [no] <field> /<regexp>/  { ... }`

  Executes all nested steps if the condition specified in the directive is true
  (field contains the specified substring).

  If the substring is wrapped in forward slashes - it will be interpreted as a
  Perl-compatible regular expression that should match field contents.

  Valid "fields":
  - `rcpt`
    Message recipient addresses, the condition is true if at least one
    recipient matches.
  - `from`
    Message sender address.
  - `src-addr`
    IP of the client who submitted the message.
  - `src-hostname`
    Hostname reported by the client in the EHLO/HELO command.

  See below for example.


```

smtp smtp://0.0.0.0:25 smtps://0.0.0.0:587 {
    tls /etc/ssl/private/cert.pem /etc/ssl/private/pkey.key
    auth pam
    hostname emersion.fr

    match rcpt "/@emersion.fr$/" {
        filter dkim verify
        deliver local
    }
    match no rcpt "/@emersion.fr$/" {
        require-auth
        filter dkim sign
        deliver out-queue
    }
}
```

### 'sqlmail' module

SQL-based storage backend.  Can be used as a storage backend (for IMAP),
authentication provider (IMAP & SMTP) or delivery target (SMTP).

Valid configuration directives:
* `driver `
  Use a specified driver to communicate with the database.  Supported values:
  sqlite3, mysql, postgresql.

  Latter two are not included by default and should be enabled using
  corresponding build tags.

* `dsn `
  Data Source Name, the driver-specific value that specifies the database to use.
  For SQLite3 this is just a file path.
  For MySQL: https://github.com/go-sql-driver/mysql#dsn-data-source-name
  For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

### 'dummy' module

No-op module. It doesn't need to be configured explicitly and can be referenced
using `dummy` name. It can act as a filter, delivery target, and auth.
provider.  In the latter case, it will accept any credentials, allowing any
client to authenticate using any username and password (use with care!).
