# LDAP BindDN

maddy supports authentication via LDAP using DN binding. Passwords are verified
by the LDAP server.

maddy needs to know the DN to use for binding. It can be obtained either by
directory search or template .

Note that storage backends conventionally use email addresses, if you use
non-email identifiers as usernames then you should map them onto
emails on delivery by using auth\_map (see documentation page for used storage backend).

auth.ldap also can be a used as a table module. This way you can check
whether the account exists. It works only if DN template is not used.

```
auth.ldap {
    urls ldap://maddy.test:389

    # Specify initial bind credentials. Not required ('bind off')
    # if DN template is used.
    bind plain "cn=maddy,ou=people,dc=maddy,dc=test" "123456"

    # Specify DN template to skip lookup.
    dn_template "cn={username},ou=people,dc=maddy,dc=test"

    # Specify base_dn and filter to lookup DN.
    base_dn "ou=people,dc=maddy,dc=test"
    filter "(&(objectClass=posixAccount)(uid={username}))"

    tls_client { ... }
    starttls off
    debug off
    connect_timeout 1m
}
```
```
auth.ldap ldap://maddy.test.389 {
    ...
}
```

## Configuration directives

**Syntax:** urls _servers...\_

REQUIRED.

URLs of the directory servers to use. First available server
is used - no load-balancing is done.

URLs should use 'ldap://', 'ldaps://', 'ldapi://' schemes.

**Syntax:** bind off <br>
bind unauth <br>
bind external <br>
bind plain _username_ _password_ <br>
**Default:** off

Credentials to use for initial binding. Required if DN lookup is used.

'unauth' performs unauthenticated bind. 'external' performs external binding
which is useful for Unix socket connections (ldapi://) or TLS client certificate
authentication (cert. is set using tls\_client directive). 'plain' performs a
simple bind using provided credentials.

**Syntax:** dn\_template _template\_

DN template to use for binding. '{username}' is replaced with the
username specified by the user.

**Syntax:** base\_dn _dn\_

Base DN to use for lookup.

**Syntax:** filter _str\_

DN lookup filter. '{username}' is replaced with the username specified
by the user.

Example:
```
(&(objectClass=posixAccount)(uid={username}))
```

Example (using ActiveDirectory):
```
(&(objectCategory=Person)(memberOf=CN=user-group,OU=example,DC=example,DC=org)(sAMAccountName={username})(!(UserAccountControl:1.2.840.113556.1.4.803:=2)))
```

Example:
```
(&(objectClass=Person)(mail={username}))
```

**Syntax:** starttls _bool_ <br>
**Default:** off

Whether to upgrade connection to TLS using STARTTLS.

**Syntax:** tls\_client { ... }

Advanced TLS client configuration. See [TLS configuration / Client](/reference/tls/#client) for details.

**Syntax:** connect\_timeout _duration_ <br>
**Default:** 1m

Timeout for initial connection to the directory server.

**Syntax:** request\_timeout _duration_ <br>
**Default:** 1m

Timeout for each request (binding, lookup).
