# Multiple domains configuration

By default, maddy uses email addresses as account identifiers for both
authentication and storage purposes. Therefore, account named `user@example.org`
is completely independent from `user@example.com`. They must be created
separately, may have different credentials and have separate IMAP mailboxes.

This makes it extremely easy to setup maddy to manage multiple otherwise
independent domains.

Default configuration file contains two macros - `$(primary_domain)` and
`$(local_domains)`. They are used to used in several places thorough the
file to configure message routing, security checks, etc.

In general, you should just add all domains you want maddy to manage to
`$(local_domains)`, like this:
```
$(primary_domain) = example.org
$(local_domains) = $(primary_domain) example.com
```
Note that you need to pick one domain as a "primary" for use in
auto-generated messages.

With that done, you can create accounts using both domains in the name, send
and receive messages and so on.  Do not forget to configure corresponding SPF,
DMARC and MTA-STS records as was recommended in
the [introduction tutorial](tutorials/setting-up.md).

Also note that you do not really need a separate TLS certificate for each
managed domain. You can have one hostname e.g. mail.example.org set as an MX
record for multiple domains.

**If you want multiple domains to share username namespace**, you should change
several more options.

You can make "user@example.org" and "user@example.com" users share the same
credentials of user "user" but have different IMAP mailboxes ("user@example.org"
and "user@example.com" correspondingly). For that, it is enough to set `auth_map`
globally to use `email_localpart` table:
```
auth_map email_localpart
```
This way, when user logs in as "user@example.org", "user" will be passed
to the authentication provider, but "user@example.org" will be passed to the
storage backend. You should create accounts like this:
```
maddy creds create user
maddy imap-acct create user@example.org
maddy imap-acct create user@example.com
```

**If you want accounts to also share the same IMAP storage of account named
"user"**, you can set `storage_map` in IMAP endpoint and `delivery_map` in
storage backend to use `email_locapart`:
```
storage.imapsql local_mailboxes {
   ...
   delivery_map email_localpart # deliver "user@*" to "user"
}
imap tls://0.0.0.0:993 {
   ...
   storage &local_mailboxes
   ...
   storage_map email_localpart # "user@*" accesses "user" mailbox
}
```

You also might want to make it possible to log in without
specifying a domain at all. In this case, use `email_localpart_optional` for
both `auth_map` and `storage_map`.

You also need to make `authorize_sender` check (used in `submission` endpoint)
accept non-email usernames:
```
authorize_sender {
  ...
  user_to_email chain {
    step email_localpart_optional           # remove domain from username if present
    step email_with_domain $(local_domains) # expand username with all allowed domains
  }
}
```

## TL;DR

Your options:

**"user@example.org" and "user@example.com" have distinct credentials and
distinct mailboxes.**

```
$(primary_domain) = example.org
$(local_domains) = example.org example.com
```

Create accounts as:

```shell
maddy creds create user@example.org
maddy imap-acct create user@example.org
maddy creds create user@example.com
maddy imap-acct create user@example.com
```

**"user@example.org" and "user@example.com" have same credentials but
distinct mailboxes.**

```
$(primary_domain) = example.org
$(local_domains) = example.org example.com
auth_map email_localpart
```

Create accounts as:
```shell
maddy creds create user
maddy imap-acct create user@example.org
maddy imap-acct create user@example.com
```

**"user@example.org", "user@example.com", "user" have same credentials and same
mailboxes.**

```
   $(primary_domain) = example.org
   $(local_domains) = example.org example.com
   auth_map email_localpart_optional # authenticating as "user@*" checks credentials for "user"

   storage.imapsql local_mailboxes {
      ...
      delivery_map email_localpart_optional # deliver "user@*" to "user" mailbox
   }

   imap tls://0.0.0.0:993 {
      ...
      storage_map email_localpart_optional # authenticating as "user@*" accesses "user" mailboxes
   }

   submission tls://0.0.0.0:465 {
      check {
        authorize_sender {
          ...
          user_to_email chain {
            step email_localpart_optional           # remove domain from username if present
            step email_with_domain $(local_domains) # expand username with all allowed domains
          }
        }
      }
      ...
   }
```

Create accounts as:
```shell
maddy creds create user
maddy imap-acct create user
```
