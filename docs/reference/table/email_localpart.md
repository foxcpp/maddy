# Email local part

The module 'table.email\_localpart' extracts and unescaped local ("username") part
of the email address.

E.g.
test@example.org => test
"test @ a"@example.org => test @ a

```
table.email_localpart { }
```
