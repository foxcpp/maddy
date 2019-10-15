maddy-targets(5) "maddy mail server - storage backends"

# INTRODUCTION

This man page describes modules that can be used to store messages for local recipients.

Most likely, you are going to use these modules with 'storage' directive of 
IMAP endpoint (see maddy.conf(5)).

# SEE ALSO

*maddy.conf*(5) for main man page.

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
