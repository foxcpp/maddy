# SQL query mapping

The table.sql_query module implements table interface using SQL queries.

Definition:

```
table.sql_query {
	driver <driver name>
	dsn <data source name>
	lookup <lookup query>

	# Optional:
	init <init query list>
	list <list query>
	add <add query>
	del <del query>
	set <set query>
}
```

Usage example:

```
# Resolve SMTP address aliases using PostgreSQL DB.
modify {
	replace_rcpt sql_query {
		driver postgres
		dsn "dbname=maddy user=maddy"
		lookup "SELECT alias FROM aliases WHERE address = $1"
	}
}
```

## Configuration directives

### driver _driver name_ 
**Required.**

Driver to use to access the database.

Supported drivers: `postgres`, `sqlite3` (if compiled with C support)

---

### dsn _data source name_
**Required.**

Data Source Name to pass to the driver. For SQLite3 this is just a path to DB
file. For Postgres, see
[https://pkg.go.dev/github.com/lib/pq?tab=doc#hdr-Connection\_String\_Parameters](https://pkg.go.dev/github.com/lib/pq?tab=doc#hdr-Connection\_String\_Parameters)

---

### lookup _query_
**Required.**

SQL query to use to obtain the lookup result.

It will get one named argument containing the lookup key. Use :key
placeholder to access it in SQL. The result row set should contain one row, one
column with the string that will be used as a lookup result. If there are more
rows, they will be ignored. If there are more columns, lookup will fail.  If
there are no rows, lookup returns "no results". If there are any error - lookup
will fail.

---

### init _queries..._
Default: empty

List of queries to execute on initialization. Can be used to configure RDBMS.

Example, to improve SQLite3 performance:

```
table.sql_query {
	driver sqlite3
	dsn whatever.db
	init "PRAGMA journal_mode=WAL" \
		"PRAGMA synchronous=NORMAL"
	lookup "SELECT alias FROM aliases WHERE address = $1"
}
```

---

### named_args _boolean_
Default: `yes`

Whether to use named parameters binding when executing SQL queries
or not.

Note that maddy's PostgreSQL driver does not support named parameters and
SQLite3 driver has issues handling numbered parameters:
[https://github.com/mattn/go-sqlite3/issues/472](https://github.com/mattn/go-sqlite3/issues/472)

---

### add _query_<br>list _query_<br>set _query_ <br>del _query_
Default: none

If queries are set to implement corresponding table operations - table becomes
"mutable" and can be used in contexts that require writable key-value store.

'add' query gets :key, :value named arguments - key and value strings to store.
They should be added to the store. The query **should** not add multiple values
for the same key and **should** fail if the key already exists.

'list' query gets no arguments and should return a column with all keys in
the store.

'set' query gets :key, :value named arguments - key and value and should replace the existing
entry in the database.

'del' query gets :key argument - key and should remove it from the database.

If `named_args` is set to `no` - key is passed as the first numbered parameter
($1), value is passed as the second numbered parameter ($2).

