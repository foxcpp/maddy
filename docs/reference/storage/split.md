# Per-account storage routing (storage.split)

**NOTE:** `storage.split` is the new-style IMAP4rev2 storage
module that cannot be used with old imap (IMAP4rev1) endpoint.

The storage.split allows using multiple independent storage modules
for different accounts with one IMAP4rev2 endpoint.

Each storage module is given a mapping name and then a table is defined
that maps account names to storage module names. table.switch module
is particularly useful as a mapping table, as most storage modules
also support table interface - they act as a membership check table
containing account names mapped to empty strings.

storage.split does not provide management API, therefore you need to use
underlying storage modules directly to create accounts and mailboxes.

**Example:** Using split and table.switch to migrate imapsql
to imapsql2 on per-account basis.
```
storage.imapsql old_mailboxes {
    driver sqlite3
    dsn imapsql.db
}
storage.imapsql2 local_mailboxes {
    driver sqlite3
    dsn imapsql2.db
}

imap2 ... {
    storage split mapped_storage {
        mapping switch {
            case new &local_mailboxes
            case old &old_mailboxes
        }

        named new &local_mailboxes
        named old &old_mailboxes

        default &local_mailboxes
    }
}
```

Note that default module should be specified if account auto-creation is enabled.
storage.split will attempt to use 'default' module if neither of named ones match
and will therefore cause it to auto-create account if configured to do so.

## Configuration directives

```
storage.split {
    mapping ...

    named name storage_module
    named name2 storage_module2
    ...

    default storage_module3
}
```

### mapping _module-reference_
Default: not specified

Use the specified table to map account names to storage module aliases.

### named _name_ _module-reference_

Add a storage module with a given alias.

### default _module-reference_
Default: not specified

Default storage module to use if account name does not match
any of the named ones.

If there is no default module and account name does not match any of the
named ones, storage.split will return "authentication failure" error for
that account.
