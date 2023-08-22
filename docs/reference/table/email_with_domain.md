# Email with domain

The table module `table.email_with_domain` appends one or more
domains (allowing 1:N expansion) to the specified value.

```
table.email_with_domain DOMAIN DOMAIN... { }
```

It can be used to implement domain-level expansion for aliases if used together
with `table.chain`. Example:

```
modify {
    replace_rcpt chain {
        step email_local_part
        step email_with_domain example.org example.com
    }
}
```

This configuration will alias `anything@anydomain` to `anything@example.org`
and `anything@example.com`.

It is also useful with `authorize_sender` to authorize sending using multiple
addresses under different domains if non-email usernames are used for
authentication:

```
check.authorize_sender {
   ...
   user_to_email email_with_domain example.org example.com
}
```

This way, user authenticated as `user` will be allowed to use
`user@example.org` or `user@example.com` as a sender address.
