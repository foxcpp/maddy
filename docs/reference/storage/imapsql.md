# SQL-indexed storage

The imapsql module implements database for IMAP index and message
metadata using SQL-based relational database.

Message contents are stored in an "blob store" defined by msg\_store
directive. By default this is a file system directory under /var/lib/maddy.

Supported RDBMS:
- SQLite 3.25.0
- PostgreSQL 9.6 or newer
- CockroachDB 20.1.5 or newer

Account names are required to have the form of a email address (unless configured otherwise)
and are case-insensitive. UTF-8 names are supported with restrictions defined in the
PRECIS UsernameCaseMapped profile.

```
storage.imapsql {
	driver sqlite3
	dsn imapsql.db
	msg_store fs messages/
}
```

imapsql module also can be used as a lookup table.
It returns empty string values for existing usernames. This might be useful
with destination\_in directive e.g. to implement catch-all
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

**Syntax**: driver _string_ <br>
**Default**: not specified

REQUIRED.

Use a specified driver to communicate with the database. Supported values:
sqlite3, postgres.

Should be specified either via an argument or via this directive.

**Syntax**: dsn _string_ <br>
**Default**: not specified

REQUIRED.

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.
For PostgreSQL: [https://godoc.org/github.com/lib/pq#hdr-Connection\_String\_Parameters](https://godoc.org/github.com/lib/pq#hdr-Connection\_String\_Parameters)

Should be specified either via an argument or via this directive.

**Syntax**: msg\_store _store_ <br>
**Default**: fs messages/

Module to use for message bodies storage.

See "Blob storage" section for what you can use here.

**Syntax**: <br>
compression off <br>
compression _algorithm_ <br>
compression _algorithm_ _level_ <br>
**Default**: off

Apply compression to message contents.
Supported algorithms: lz4, zstd.

**Syntax**: appendlimit _size_ <br>
**Default**: 32M

Don't allow users to add new messages larger than 'size'.

This does not affect messages added when using module as a delivery target.
Use 'max\_message\_size' directive in SMTP endpoint module to restrict it too.

**Syntax**: debug _boolean_ <br>
**Default**: global directive value

Enable verbose logging.

**Syntax**: junk\_mailbox _name_ <br>
**Default**: Junk

The folder to put quarantined messages in. Thishis setting is not used if user
does have a folder with "Junk" special-use attribute.

**Syntax**: disable\_recent _boolean_ <br>
*Default: true

Disable RFC 3501-conforming handling of \Recent flag.

This significantly improves storage performance when SQLite3 or CockroackDB is
used at the cost of confusing clients that use this flag.

**Syntax**: sqlite\_cache\_size _integer_ <br>
**Default**: defined by SQLite

SQLite page cache size. If positive - specifies amount of pages (1 page - 4
KiB) to keep in cache. If negative - specifies approximate upper bound
of cache size in KiB.

**Syntax**: sqlite\_busy\_timeout _integer_ <br>
**Default**: 5000000

SQLite-specific performance tuning option. Amount of milliseconds to wait
before giving up on DB lock.

**Syntax**: imap\_filter { ... } <br>
**Default**: not set

Specifies IMAP filters to apply for messages delivered from SMTP pipeline.

Ex.
```
imap_filter {
	command /etc/maddy/sieve.sh {account_name}
}
```

**Syntax:** delivery\_map **table** <br>
**Default:** identity

Use specified table module to map recipient
addresses from incoming messages to mailbox names.

Normalization algorithm specified in delivery\_normalize is appied before
delivery\_map.

**Syntax:** delivery\_normalize _name_ <br>
**Default:** precis\_casefold\_email

Normalization function to apply to email addresses before mapping them
to mailboxes.

See auth\_normalize.

**Syntax**: auth\_map **table** <br>
**Default**: identity

Use specified table module to map authentication
usernames to mailbox names.

Normalization algorithm specified in auth\_normalize is applied before
auth\_map.

**Syntax**: auth\_normalize _name_ <br>
**Default**: precis\_casefold\_email

Normalization function to apply to authentication usernames before mapping
them to mailboxes.

Available options:
- precis\_casefold\_email   PRECIS UsernameCaseMapped profile + U-labels form for domain
- precis\_casefold         PRECIS UsernameCaseMapped profile for the entire string
- precis\_email            PRECIS UsernameCasePreserved profile + U-labels form for domain
- precis                  PRECIS UsernameCasePreserved profile for the entire string
- casefold                Convert to lower case
- noop                    Nothing

Note: On message delivery, recipient address is unconditionally normalized
using precis\_casefold\_email function.

