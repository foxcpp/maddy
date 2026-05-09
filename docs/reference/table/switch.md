# Switch table (table.switch)

The `table.switch` module routes table lookups across multiple sub-tables.
For each incoming key, each `case` is tried in order: the key is looked up in
the case's associated table, and if a match is found the fixed `value` string
defined for that case is returned. If no case matches, the optional `default`
value is returned.

This is particularly useful with `storage.split` to map account names to named
storage backends:

```
table.switch {
    case new &local_mailboxes
    case old &old_mailboxes
    default new
}
```

In the example above, if the key is found in `local_mailboxes` (i.e. the
account exists there), the string `"new"` is returned. If it is found in
`old_mailboxes`, `"old"` is returned. For keys not found in either table,
`"new"` is returned as a fallback.

## Configuration directives

```
table.switch {
    case <value> <table-ref>
    default <value>
    allow_multiple no
}
```

---

### case _value_ _table-reference_

Register a case. When a lookup key is found in `table-reference`, the fixed
string `value` is returned instead of the key itself.

Multiple `case` directives may be specified and are evaluated in declaration
order. The first matching case wins unless `allow_multiple` is enabled.

The `table-reference` can be an inline module definition or a reference to a
named module instance (e.g. `&my_table`).

---

### default _value_
Default: not set

The value to return when no `case` matches the lookup key.

If `default` is not set and no case matches, the key is treated as not present
in the table (lookup returns no value).

If `allow_multiple` is enabled, `default` is returned only if no cases match.

---

### allow_multiple _boolean_
Default: `no`

When set to `yes`, all matching cases are evaluated instead of stopping at
the first match.

If table is used in a single-value context, the last matching case wins. If used in a
multi-value context, all matching case values are returned.

