maddy-targets(5) "maddy mail server" "maddy reference documentation"

; TITLE Storage backends

maddy storage interface is built with IMAP in mind and directly represents
IMAP data model. That is, maddy storage does have the concept of folders,
flags, message UIDs, etc defined as in RFC 3501.

This man page lists supported storage backends along with supported
configuration directives for each.

Most likely, you are going to modules listed here in 'storage' directive for
IMAP endpoint module (see *maddy-imap*(5)).

# SQL-based database module (sql)

Module that stores message metadata and indexes in a SQL-based relational
database.

Message contents are stored in an "external store", currently the only
supported "external store" is a filesystem directory, enabled by 'fsstore'
directive.

Supported RDBMS:
- SQLite 3.25.0
- PostgreSQL 9.6 or newer

Can be used as a storage backend (for IMAP), authentication backend (IMAP &
SMTP) or delivery target (SMTP).

See https://github.com/foxcpp/go-imap-sql for more information.

## Configuration directives

*Syntax*: driver _string_ ++
*Default*: not specified

REQUIRED.

Use a specified driver to communicate with the database. Supported values:
sqlite3, postgres.

*Syntax*: dsn _string_ ++
*Default*: not specified

REQUIRED.

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.
For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

*Syntax*: ++
    fsstore ++
    fsstore _directory_ ++
*Default*: not specified

REQUIRED.

Store message contents in filesystem directory.

If directory path is not specified - it will be derived from configuration
block name ('StateDirectory/sql-NAME-fsstore').

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

*Syntax*: sqlite_exclusive_lock _boolean_ ++
*Default*: no

SQLite-specific performance tuning option. Slightly decereases ovehead of
DB locking at cost of making DB inaccessible for other processes (including
maddyctl utility).

*Syntax*: sqlite_cache_size _integer_ ++
*Default*: defined by SQLite

SQLite page cache size. If positive - specifies amount of pages (1 page - 4
KiB) to keep in cache. If negative - specifies approximate upper bound
of cache size in KiB.

*Syntax*: sqlite_busy_timeout _integer_ ++
*Default*: 5000000

SQLite-specific performance tuning option. Amount of milliseconds to wait
before giving up on DB lock.

*Syntax*: storage_perdomain _boolean_ ++
*Default*: no

Don't remove domain part of username when accessing the underlying storage and
require it to be present. Can be used if you want user@domain1 and user@domain2
to be different accounts at the storage level.

Typically used with 'auth_domain'.

*Syntax*: auth_perdomain _boolean_ ++
*Default*: global directive value

Don't remove domain part of username when authenticating and require it to be
present. Can be used if you want user@domain1 and user@domain2 to be different
accounts.

*Syntax*: auth_domains _domains..._ ++
*Default*: not specified

Domains that should be allowed in username during authentication.

For example, if 'auth_domains' is set to "domain1 domain2", then
username, username@domain1 and username@domain2 will be accepted as valid login
name in addition to just username.

If used without 'auth_perdomain', domain part will be removed from login before
check with underlying auth. mechanism. If 'auth_perdomain' is set, then
auth_domains must be also set and domain part WILL NOT be removed before check.
