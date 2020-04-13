maddy-tables(5) "maddy mail server" "maddy reference documentation"

; TITLE String-string translation

Whenever you need to replace one string with another when handling anything in
maddy, you can use any of the following modules to obtain the replacement
string. They are commonly called "table modules" or just "tables".

Some table modules implement write options allowing other maddy modules to
change the source of data, effectively turning the table into a complete
interface to a key-value store for maddy. Such tables are referred to as
"mutable tables".

# File mapping (file_table)

This module builds string-string mapping from a text file.

File is reloaded every 15 seconds if there are any changes (detected using
modification time). No changes are applied if file contains syntax errors.

Definition:
```
file_table <file path>
```
or
```
file_table {
	file <file path>
}
```

Usage example:
```
# Resolve SMTP address aliases using text file mapping.
modify {
	alias file_table /etc/maddy/aliases
}
```

## Syntax

Better demonstrated by examples:

```
# Lines starting with # are ignored.

# And so are lines only with whitespace.

# Whenever 'aaa' is looked up, return 'bbb'
aaa: bbb

	# Trailing and leading whitespace is ignored.
	ccc: ddd

# If there is no colon, the string is translated into ""
# That is, the following line is equivalent to
#	aaa:
aaa
```

# SQL query mapping (sql_query)

The sql_query module implements table interface using SQL queries.

Definition:
```
sql_query {
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
	alias sql_query {
		driver postgres
		dsn "dbname=maddy user=maddy"
		lookup "SELECT alias FROM aliases WHERE address = $1"
	}
}
```

## Configuration directives

**Syntax**: driver _driver name_ ++
**REQUIRED**

Driver to use to access the database.

Supported drivers: postgres, sqlite3 (if compiled with C support)

**Syntax**: dsn _data source name_ ++
**REQUIRED**

Data Source Name to pass to the driver. For SQLite3 this is just a path to DB
file. For Postgres, see
https://pkg.go.dev/github.com/lib/pq?tab=doc#hdr-Connection_String_Parameters

**Syntax**: lookup _query_ ++
**REQUIRED**

SQL query to use to obtain the lookup result.

It will get one positional argument containing the lookup key. Use $1
placeholder to access it in SQL. The result row set should contain one row, one
column with the string that will be used as a lookup result. If there are more
rows, they will be ignored. If there are more columns, lookup will fail.  If
there are no rows, lookup returns "no results". If there are any error - lookup
will fail.

**Syntax**: init _queries..._ ++
**Default**: empty

List of queries to execute on initialization. Can be used to configure RDBMS.

Example, to improve SQLite3 performance:
```
sql_query {
	driver sqlite3
	dsn whatever.db
	init "PRAGMA journal_mode=WAL" \
		"PRAGMA synchronous=NORMAL"
	lookup "SELECT alias FROM aliases WHERE address = $1"
}
```

**Syntax:** add _query_ ++
**Syntax:** list _query_ ++
**Syntax:** set _query_ ++
**Syntax:** del _query_ ++
**Default:** none

If queries are set to implement corresponding table operations - table becomes
"mutable" and can be used in contexts that require writable key-value store.

'add' query gets two ordered arguments - key and value strings to store.
They should be added to the store. The query *should* not add multiple values
for the same key and *should* fail if the key already exists.

'list' query gets no arguments and should return a column with all keys in
the store.

'set' query gets two arguments - key and value and should replace the existing
entry in the database.

'del' query gets one argument - key and should remove it from the database.

# Static table (static)

The 'static' module implements table lookups using key-value pairs in its
configuration.

```
static {
	entry KEY1 VALUE1
	entry KEY2 VALUE2
	...
}
```

## Configuration directives

**Syntax**: entry _key_ _value_

Add an entry to the table.

If the same key is used multiple times, the last one takes effect.

# Regexp rewrite table (regexp)

The 'regexp' module implements table lookups by applying a regular expression
to the key value. If it matches - 'replacement' value is returned with $N
placeholders being replaced with corresponding capture groups from the match.
Otherwise, no value is returned.

The regular expression syntax is the subset of PCRE. See
https://golang.org/pkg/regexp/syntax/ for details.

```
regexp <regexp> <replacement> {
	full_match yes
	case_insensitive yes
	expand_placeholders yes
}
```

## Configuration directives

**Syntax**: full_match _boolean_ ++
**Default**: yes

Whether to implicitly add start/end anchors to the regular expression.
That is, if 'full_match' is yes, then the provided regular expression should
match the whole string. With no - partial match is enough.

**Syntax**: case_insensitive _boolean_ ++
**Default**: yes

Whether to make matching case-insensitive.

**Syntax**: expand_placeholders _boolean_ ++
**Default**: yes

Replace '$name' and '${name}' in the replacement string with contents of
corresponding capture groups from the match.

To insert a literal $ in the output, use $$ in the template.

# Identity table (identity)

The module 'identity' is a table module that just returns the key looked up.

```
identity { }
```

# No-op table (dummy)

The module 'dummy' represents an empty table.

```
dummy { }
```
