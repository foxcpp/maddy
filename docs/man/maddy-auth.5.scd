maddy-auth(5) "maddy mail server" "maddy authentication backends"

; TITLE Authentication backends

# Introduction

Modules described in this man page can be used to provide functionality to
check validity of username-password pairs in accordance with some database.
That is, they authenticate users.

Most likely, you are going to use these modules with 'auth' directive of IMAP
(*maddy-imap*(5)) or SMTP endpoint (*maddy-smtp*(5)).

# SQL module (sql)

sql module described in *maddy-storage*(5) can also be used as a authentication
backend.

# External authentication module (extauth)

Module for authentication using external helper binary. It looks for binary
named maddy-auth-helper in $PATH and libexecdir and uses it for authentication.

The protocol is very simple:
Program is launched for each authentication. Username and password are written
to stdin, adding \\n to the end. If binary exits with 0 status code -
authentication is considered successful. If the status code is 1 -
authentication is failed. If the status code is 2 - another unrelated error has
happened. Additional information should be written to stderr.

```
extauth {
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

# PAM module (pam)

Implements authentication using libpam. Alternatively it can be configured to
use helper binary like extauth module does.

maddy should be built with libpam build tag to use this module without
'use_helper' directive.
```
go get -tags 'libpam' ...
```

By default, when checking with the PAM database, the username provided by
client is expected to be a RFC 5321 e-mail address and the domain part and
at-sign is splitten. To disable that behavior and pass username as-is to
libpam, set 'expect_address' to 'no'. Note that currently implemented storage
backends require full e-mail address as an account name, so this is still not
possible to use accounts with non-address names.

```
pam {
    debug no
    use_helper no
	expect_address yes
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

TL;DR (assuming you have maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

# Shadow database authentication module (shadow)

Implements authentication by reading /etc/shadow. Alternatively it can be
configured to use helper binary like extauth does.

When checking with the PAM database, the username provided by client is
expected to be a RFC 5321 e-mail address and the domain part and at-sign is
removed.

```
shadow {
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
