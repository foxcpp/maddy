# SQL-indexed storage (version 2, EXPERIMENTAL)

The storage.imapsql2 module implements database for IMAP index and message
metadata using SQL-based relational database.

Supported RDBMS:
- SQLite 3.25.0 (currently only native C version)

Each stored message is decomposed into text parts and attachments, which are
stored separately as blobs using provided `blob_storage`. This allows faster
partial message access (as typically used by IMAP clients) at the cost of losing
exact byte-to-byte representation of the message. In particular, MIME multipart
prologue and epilogue parts are discarded, this can lead to invalidation
of digital signatures on messages with deeply nested MIME structure.

storage.imapsql2 module also can be used as a lookup table.
It returns empty string values for existing usernames. This might be useful
with `destination_in` directive e.g. to implement catch-all
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

```
storage.imapsql2 {
    driver sqlite3
    dsn imapsql.db
    blob_store fs messages/
    temp_store fs buffer/
    auto_create_accounts true
    appendlimit 32M
    debug global
    imap_filter ...
    delivery_map identity
    delivery_normalize precis_casefold_email
}
```

### driver _string_
**Required.**<br>
Default: not specified

Use a specified driver to communicate with the database. Supported values:
sqlite3, sqlite (same as sqlite3).

Should be specified either via an argument or via this directive.

---

### dsn _string_
**Required.**<br>
Default: not specified

Data Source Name, the driver-specific value that specifies the database to use.

For SQLite3 this is just a file path.

Should be specified either via an argument or via this directive.

---

### blob_store _store_
Default: `fs messages/`

Module to use for message bodies storage.

See "Blob storage" section for what you can use here.

---

### temp_store _store_
Default: `fs buffer/`

Module to use to temporarily store appended messages before they are processed
and stored in `blob_store`.

This applies only to messages added via IMAP APPEND command, messages delivered from SMTP
pipeline are buffered by SMTP endpoint according to its `buffer` directive.

---

### auto_create_accounts _boolean_
Default: `true`

If enabled, storage accounts will be automatically created when first accessed
via IMAP with successful authentication.

---

### appendlimit _size_
Default: `32M`

This does not affect messages added when using module as a delivery target.
Use `max_message_size` directive in SMTP endpoint module to restrict it too.

---

### debug _boolean_
Default: global directive value

Enable verbose logging.

---

### imap_filter { ... }
Default: not set

Specifies IMAP filters to apply for messages delivered from SMTP pipeline.

Example:
```
imap_filter {
	command /etc/maddy/sieve.sh {account_name}
}
```

---

### delivery_map _table_
Default: not set (use normalized address directly)

Use specified table module to map recipient
addresses from incoming messages to mailbox names.

Normalization algorithm specified in `delivery_normalize` is applied before
`delivery_map`.

---

### delivery_normalize _name_
Default: `precis_casefold_email`

Normalization function to apply to email addresses before mapping them
to mailboxes.

Available options:

- `precis_casefold_email`   PRECIS UsernameCaseMapped profile + U-labels form for domain
- `precis_casefold`         PRECIS UsernameCaseMapped profile for the entire string
- `precis_email`            PRECIS UsernameCasePreserved profile + U-labels form for domain
- `precis`                  PRECIS UsernameCasePreserved profile for the entire string
- `casefold`                Convert to lower case
- `noop`                    Nothing
