maddy-auth(5) "maddy mail server" "maddy authentication backends"

; TITLE Authentication backends

# Introduction

Modules described in this man page can be used to provide functionality to
check validity of username-password pairs in accordance with some database.
That is, they authenticate users.

Most likely, you are going to use these modules with 'auth' directive of IMAP
(*maddy-imap*(5)) or SMTP endpoint (*maddy-smtp*(5)).

Most modules listed here are also usable as a table (see *maddy-tables*(5))
that contains all usernames known to the module. Exceptions are auth.external and
pam as underlying interfaces do not define a way to check credentials
existence.

# External authentication module (auth.external)

Module for authentication using external helper binary. It looks for binary
named maddy-auth-helper in $PATH and libexecdir and uses it for authentication
using username/password pair.

The protocol is very simple:
Program is launched for each authentication. Username and password are written
to stdin, adding \\n to the end. If binary exits with 0 status code -
authentication is considered successful. If the status code is 1 -
authentication is failed. If the status code is 2 - another unrelated error has
happened. Additional information should be written to stderr.

```
auth.external {
    helper /usr/bin/ldap-helper
    perdomain no
    domains example.org
}
```

## Configuration directives

*Syntax*: helper _file_path_

Location of the helper binary. *Required.*

*Syntax*: perdomain _boolean_ ++
*Default*: no

Don't remove domain part of username when authenticating and require it to be
present. Can be used if you want user@domain1 and user@domain2 to be different
accounts.

*Syntax*: domains _domains..._ ++
*Default*: not specified

Domains that should be allowed in username during authentication.

For example, if 'domains' is set to "domain1 domain2", then
username, username@domain1 and username@domain2 will be accepted as valid login
name in addition to just username.

If used without 'perdomain', domain part will be removed from login before
check with underlying auth. mechanism. If 'perdomain' is set, then
domains must be also set and domain part WILL NOT be removed before check.

# PAM module (auth.pam)

Implements authentication using libpam. Alternatively it can be configured to
use helper binary like auth.external module does.

maddy should be built with libpam build tag to use this module without
'use_helper' directive.
```
go get -tags 'libpam' ...
```

```
auth.pam {
    debug no
    use_helper no
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: no

Enable verbose logging for all modules. You don't need that unless you are
reporting a bug.

*Syntax*: use_helper _boolean_ ++
*Default*: no

Use LibexecDirectory/maddy-pam-helper instead of directly calling libpam.
You need to use that if:
1. maddy is not compiled with libpam, but maddy-pam-helper is built separately.
2. maddy is running as an unprivileged user and used PAM configuration requires additional
privileges (e.g. when using system accounts).

For 2, you need to make maddy-pam-helper binary setuid, see
README.md in source tree for details.

TL;DR (assuming you have the maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

# Shadow database authentication module (auth.shadow)

Implements authentication by reading /etc/shadow. Alternatively it can be
configured to use helper binary like auth.external does.

```
auth.shadow {
    debug no
    use_helper no
}
```

## Configuration directives

*Syntax*: debug _boolean_ ++
*Default*: no

Enable verbose logging for all modules. You don't need that unless you are
reporting a bug.

*Syntax*: use_helper _boolean_ ++
*Default*: no

Use LibexecDirectory/maddy-shadow-helper instead of directly reading /etc/shadow.
You need to use that if maddy is running as an unprivileged user
privileges (e.g. when using system accounts).

You need to make maddy-shadow-helper binary setuid, see
cmd/maddy-shadow-helper/README.md in source tree for details.

TL;DR (assuming you have maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-shadow-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-shadow-helper
```

# Table-based password hash lookup (auth.pass_table)

This module implements username:password authentication by looking up the
password hash using a table module (maddy-tables(5)). It can be used
to load user credentials from text file (file module) or SQL query
(sql_table module).


Definition:
```
auth.pass_table [block name] {
	table <table config>

}
```
Shortened variant for inline use:
```
pass_table <table> [table arguments] {
	[additional table config]
}
```

Example, read username:password pair from the text file:
```
smtp tcp://0.0.0.0:587 {
	auth pass_table file /etc/maddy/smtp_passwd
	...
}
```

## Password hashes

pass_table expects the used table to contain certain structured values with
hash algorithm name, salt and other necessary parameters.

You should use 'maddyctl hash' command to generate suitable values.
See 'maddyctl hash --help' for details.

## maddyctl creds

If the underlying table is a "mutable" table (see maddy-tables(5)) then
the 'maddyctl creds' command can be used to modify the underlying tables
via pass_table module. It will act a "local credentials store" and will write
appropriate hash values to the table.

# Separate username and password lookup (auth.plain_separate)

This module implements authentication using username:password pairs but can
use zero or more "table modules" (maddy-tables(5)) and one or more
authentication providers to verify credentials.

```
auth.plain_separate {
	user ...
	user ...
	...
	pass ...
	pass ...
	...
}
```

How it works:
- Initial username input is normalized using PRECIS UsernameCaseMapped profile.
- Each table specified with the 'user' directive looked up using normalized
	username. If match is not found in any table, authentication fails.
- Each authentication provider specified with the 'pass' directive is tried.
	If authentication with all providers fails - an error is returned.

## Configuration directives

**Syntax:** user _table module_

Configuration block for any module from maddy-tables(5) can be used here.

Example:
```
user file /etc/maddy/allowed_users
```

**Syntax:** pass _auth provider_

Configuration block for any auth. provider module can be used here, even
'plain_split' itself.

The used auth. provider must provide username:password pair-based
authentication.

# Dovecot authentication client (auth.dovecot_sasl)

The 'dovecot_sasl' module implements the client side of the Dovecot
authentication protocol, allowing maddy to use it as a credentials source.

Currently SASL mechanisms support is limited to mechanisms supported by maddy
so you cannot get e.g. SCRAM-MD5 this way.

```
auth.dovecot_sasl {
	endpoint unix://socket_path
}

dovecot_sasl unix://socket_path
```

## Configuration directives

*Syntax*: endpoint _schema://address_ ++
*Default*: not set

Set the address to use to contact Dovecot SASL server in the standard endpoint
format.

tcp://10.0.0.1:2222 for TCP, unix:///var/lib/dovecot/auth.sock for Unix
domain sockets.

# LDAP BindDN authentication (EXPERIMENTAL) (auth.ldap)

maddy supports authentication via LDAP using DN binding. Passwords are verified
by the LDAP server.

maddy needs to know the DN to use for binding. It can be obtained either by
directory search or template .

Note that storage backends conventionally use email addresses, if you use
non-email identifiers as usernames then you should map them onto
emails on delivery by using auth_map (see *maddy-storage*(5)).

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

*Syntax:* urls _servers..._

REQUIRED.

URLs of the directory servers to use. First available server
is used - no load-balancing is done.

URLs should use 'ldap://', 'ldaps://', 'ldapi://' schemes.

*Syntax:* bind off ++
    bind unauth ++
    bind external ++
    bind plain _username_ _password_ ++
*Default:* off

Credentials to use for initial binding. Required if DN lookup is used.

'unauth' performs unauthenticated bind. 'external' performs external binding
which is useful for Unix socket connections (ldapi://) or TLS client certificate
authentication (cert. is set using tls_client directive). 'plain' performs a
simple bind using provided credentials.

*Syntax:* dn_template _template_

DN template to use for binding. '{username}' is replaced with the
username specified by the user.

*Syntax:* base_dn _dn_

Base DN to use for lookup.

*Syntax:* filter _str_

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

*Syntax:* starttls _bool_ ++
*Default:* off

Whether to upgrade connection to TLS using STARTTLS.

*Syntax:* tls_client { ... }

Advanced TLS client configuration. See *maddy-tls*(5) for details.

*Syntax:* connect_timeout _duration_ ++
*Default:* 1m

Timeout for initial connection to the directory server.

*Syntax:* request_timeout _duration_ ++
*Default:* 1m

Timeout for each request (binding, lookup).