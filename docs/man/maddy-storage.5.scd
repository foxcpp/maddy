maddy-targets(5) "maddy mail server" "maddy reference documentation"

; TITLE Storage backends

maddy storage interface is built with IMAP in mind and directly represents
IMAP data model. That is, maddy storage does have the concept of folders,
flags, message UIDs, etc defined as in RFC 3501.

This man page lists supported storage backends along with supported
configuration directives for each.

Most likely, you are going to use modules listed here in 'storage' directive
for IMAP endpoint module (see *maddy-imap*(5)).

In most cases, local storage modules will auto-create accounts when they are
accessed via IMAP. This relies on authentication provider used by IMAP endpoint
to provide what essentially is access control. There is a caveat, however: this
auto-creation will not happen when delivering incoming messages via SMTP as
there is no authentication to confirm that this account should indeed be
created.

# SQL-based database module (storage.imapsql)

The imapsql module implements database for IMAP index and message
metadata using SQL-based relational database.

Message contents are stored in an "external store" defined by msg_store
directive. By default this is a file system directory under /var/lib/maddy.

Supported RDBMS:
- SQLite 3.25.0
- PostgreSQL 9.6 or newer

Account names are required to have the form of a email address and are
case-insensitive. UTF-8 names are supported with restrictions defined in the
PRECIS UsernameCaseMapped profile.

```
storage.imapsql {
	driver sqlite3
	dsn imapsql.db
	msg_store fs messages/
}
```

imapsql module also can be used as a lookup table (*maddy-table*(5)).
It returns empty string values for existing usernames. This might be useful
with destination_in directive (*maddy-smtp*(5)) e.g. to implement catch-all
addresses (this is a bad idea to do so, this is just an example):
```
destination_in &local_mailboxes {
	deliver_to &local_mailboxes
}
destination example.org {
	modify {
		replace_rcpt regexp ".*" "catchall@example.org"
	}
	deliver_to &local_mailboxes
}
```


## Arguments

Specify the driver and DSN.

## Configuration directives

*Syntax*: driver _string_ ++
*Default*: not specified

REQUIRED.

Use a specified driver to communicate with the database. Supported values:
sqlite3, postgres.

Should be specified either via an argument or via this directive.

*Syntax*: dsn _string_ ++
*Default*: not specified

REQUIRED.

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.
For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

Should be specified either via an argument or via this directive.

*Syntax*: msg_store _store_ ++
*Default*: fs messages/

Module to use for message bodies storage.

See *maddy-blob*(5) for details.

*Syntax*: ++
    compression off ++
    compression _algorithm_ ++
    compression _algorithm_ _level_ ++
*Default*: off

Apply compression to message contents.
Supported algorithms: lz4, zstd.

*Syntax*: appendlimit _size_ ++
*Default*: 32M

Don't allow users to add new messages larger than 'size'.

This does not affect messages added when using module as a delivery target.
Use 'max_message_size' directive in SMTP endpoint module to restrict it too.

*Syntax*: debug _boolean_ ++
*Default*: global directive value

Enable verbose logging.

*Syntax*: junk_mailbox _name_ ++
*Default*: Junk

The folder to put quarantined messages in. Thishis setting is not used if user
does have a folder with "Junk" special-use attribute.

*Syntax*: disable_recent _boolean_ ++
*Default: true

Disable RFC 3501-conforming handling of \Recent flag.

This significantly improves storage performance when SQLite3 or CockroackDB is
used at the cost of confusing clients that use this flag.

*Syntax*: sqlite_cache_size _integer_ ++
*Default*: defined by SQLite

SQLite page cache size. If positive - specifies amount of pages (1 page - 4
KiB) to keep in cache. If negative - specifies approximate upper bound
of cache size in KiB.

*Syntax*: sqlite_busy_timeout _integer_ ++
*Default*: 5000000

SQLite-specific performance tuning option. Amount of milliseconds to wait
before giving up on DB lock.

*Syntax*: imap_filter { ... } ++
*Default*: not set

Specifies IMAP filters to apply for messages delivered from SMTP pipeline.

See *maddy-imap*(5) for filter modules usable here.

Ex.
```
imap_filter {
	command /etc/maddy/sieve.sh {account_name}
}
```

*Syntax:* delivery_map *table* ++
*Default:* identity

Use specified table module (*maddy-tables*(5)) to map recipient
addresses from incoming messages to mailbox names.

Normalization algorithm specified in delivery_normalize is appied before
delivery_map.

*Syntax:* delivery_normalize _name_ ++
*Default:* precis_casefold_email

Normalization function to apply to email addresses before mapping them
to mailboxes.

See auth_normalize.

*Syntax*: auth_map *table* ++
*Default*: identity

Use specified table module (*maddy-tables*(5)) to map authentication
usernames to mailbox names.

Normalization algorithm specified in auth_normalize is applied before
auth_map.

*Syntax*: auth_normalize _name_ ++
*Default*: precis_casefold_email

Normalization function to apply to authentication usernames before mapping
them to mailboxes.

Available options:
- precis_casefold_email   PRECIS UsernameCaseMapped profile + U-labels form for domain
- precis_casefold         PRECIS UsernameCaseMapped profile for the entire string
- precis_email            PRECIS UsernameCasePreserved profile + U-labels form for domain
- precis                  PRECIS UsernameCasePreserved profile for the entire string
- casefold                Convert to lower case
- noop                    Nothing

Note: On message delivery, recipient address is unconditionally normalized
using precis_casefold_email function.
