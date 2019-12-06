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
supported "external store" is a filesystem directory, used by default.
All messages are stored in StateDirectory/messages under random IDs.

Supported RDBMS:
- SQLite 3.25.0
- PostgreSQL 9.6 or newer

Can be used as a storage backend (for IMAP), authentication backend (IMAP &
SMTP) or delivery target (SMTP).

Account names are required to have the form of a email address and are
case-insensitive. UTF-8 names are supported with restrictions defined in the
PRECIS UsernameCaseMapped profile.

The database can be managed using the maddyctl utility.

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

*Syntax*: fsstore _directory_ ++
*Default*: messages/

Directory to store message contents in.

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
