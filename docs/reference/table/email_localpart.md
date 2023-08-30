# Email local part

The module `table.email_localpart` extracts and unescapes local ("username") part
of the email address.

E.g.

* `test@example.org` => `test`
* `"test @ a"@example.org` => `test @ a`

Mappings for invalid emails are not defined (will be treated as non-existing
values).

```
table.email_localpart { }
```

`table.email_localpart_optional` works the same, but returns non-email strings
as is. This can be used if you want to accept both `user@example.org` and
`user` somewhere and treat it the same.
