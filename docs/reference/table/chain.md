# Table chaining

The table.chain module allows chaining together multiple table modules
by using value returned by a previous table as an input for the second
table.

Example:
```
table.chain {
	step regexp "(.+)(\\+[^+"@]+)?@example.org" "$1@example.org"
	step file /etc/maddy/emails
}
```
This will strip +prefix from mailbox before looking it up
in /etc/maddy/emails list.

## Configuration directives

### step _table_

Adds a table module to the chain. If input value is not in the table
(e.g. file) - return "not exists" error.

---

### optional_step _table_

Same as step but if input value is not in the table - it is passed to the
next step without changes.

Example:
Something like this can be used to map emails to usernames
after translating them via aliases map:

```
table.chain {
    optional_step file /etc/maddy/aliases
    step regexp "(.+)@(.+)" "$1"
}
```

