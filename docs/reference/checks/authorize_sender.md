# MAIL FROM and From authorization

Module check.authorize_sender verifies that envelope and header sender addresses belong
to the authenticated user. Address ownership is established via table
that maps each user account to a email address it is allowed to use.
There are some special cases, see user\_to\_email description below.

```
check.authorize_sender {
    prepare_email identity
    user_to_email identity
    check_header yes

    unauth_action reject
    no_match_action reject
    malformed_action reject
    err_action reject

    auth_normalize precis_casefold_email
    from_normalize precis_casefold_email
}
```
```
check {
    authorize_sender { ... }
}
```

## Configuration directives

**Syntax:** user\_to\_email _table_ <br>
**Default:** identity

Table to use for lookups. Result of the lookup should contain either the
domain name, the full email address or "*" string. If it is just domain - user
will be allowed to use any mailbox within a domain as a sender address.
If result contains "*" - user will be allowed to use any address.

**Syntax:** check\_header _boolean_ <br>
**Default:** yes

Whether to verify header sender in addition to envelope.

Either Sender or From field value should match the
authorization identity.

**Syntax:** unauth\_action _action_ <br>
**Default:** reject

What to do if the user is not authenticated at all.

**Syntax:** no\_match\_action _action_ <br>
**Default:** reject

What to do if user is not allowed to use the sender address specified.

**Syntax:** malformed\_action _action_ <br>
**Default:** reject

What to do if From or Sender header fields contain malformed values.

**Syntax:** err\_action _action_ <br>
**Default:** reject

What to do if error happens during prepare\_email or user\_to\_email lookup.

**Syntax:** auth\_normalize _action_ <br>
**Default:** precis\_casefold\_email

Normalization function to apply to authorization username before
further processing.

Available options:
- precis\_casefold\_email   PRECIS UsernameCaseMapped profile + U-labels form for domain
- precis\_casefold         PRECIS UsernameCaseMapped profile for the entire string
- precis\_email            PRECIS UsernameCasePreserved profile + U-labels form for domain
- precis                  PRECIS UsernameCasePreserved profile for the entire string
- casefold                Convert to lower case
- noop                    Nothing

**Syntax:** from\_normalize _action_ <br>
**Default:** precis\_casefold\_email

Normalization function to apply to email addresses before
further processing.

Available options are same as for auth\_normalize.
