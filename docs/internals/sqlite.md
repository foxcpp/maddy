# maddy & SQLite

SQLite is a perfect choice for small deployments because no additional
configuration is required to get started. It is recommended for cases when you
have less than 10 mail accounts.

**Note: SQLite requires DB-wide locking for writing, it means that multiple
messages can't be accepted in parallel. This is not the case for server-based
RDBMS where maddy can accept multiple messages in parallel even for a single
mailbox.**

## WAL mode

maddy forces WAL journal mode for SQLite. This makes things reasonably fast and
reduces locking contention which may be important for a typical mail server.

maddy uses increased WAL autocheckpoint interval. This means that while
maintaining a high write throughput, maddy will have to stop for a bit (0.5-1
second) every time 78 MiB is written to the DB (with default configuration it
is 15 MiB).

Note that when moving the database around you need to move WAL journal (`-wal`)
and shared memory (`-shm`) files as well, otherwise some changes to the DB will
be lost.

## Query planner statistics

maddy updates query planner statistics on shutdown and every 5 hours. It
provides query planner with information to access the database in more
efficient way because go-imap-sql schema does use a few so called "low-quality
indexes".

## Auto-vacuum

maddy turns on SQLite auto-vacuum feature. This means that database file size
will shrink when data is removed (compared to default when it remains unused).

## Manual vacuuming

Auto-vacuuming can lead to database fragmentation and thus reduce the read
performance.  To do manual vacuum operation to repack and defragment database
file, install the SQLite3 console utility and run the following commands:
```
sqlite3 -cmd 'vacuum' database_file_path_here.db
sqlite3 -cmd 'pragma wal_checkpoint(truncate)' database_file_path_here.db
```

It will take some time to complete, you can close the utility when the
`sqlite>` prompt appears.
