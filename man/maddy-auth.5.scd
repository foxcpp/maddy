maddy-auth(5) "maddy mail server - authentication backends"

# INTRODUCTION

Modules described in this man page can be used to provide functionality
to check validity of username-password pairs in accordance with some 
database. That is, they authenticate users. 

Most likely, you are going to use these modules with 'auth' directive of 
IMAP or SMTP endpoint (see maddy.conf(5)).

# SEE ALSO

*maddy.conf*(5) for basic structure description.

# SQL MODULE

sql module described in maddy-storage(5) can also be used as a authentication backend.
See mentioned man page for details.

# EXTAUTH MODULE

Module for authentication using external helper binary. It looks for binary
named maddy-auth-helper in $PATH and libexecdir and uses it for authentication.

The protocol is very simple:
Program is launched for each authentication. Username and password are written
to stdin, adding \\n to the end. If binary exits with 0 status code -
authentication is considered successful. If the status code is 1 - authentication
is failed. If the status code is 2 - another unrelated error has happened. Additional
information should be written to stderr.

```
extauth {
    helper /usr/bin/ldap-helper
}
```

Valid configuration directives:

## helper <file_path>

Location of the helper binary. Default is LibexecDirectory/maddy-auth-helper

## debug [yes/no]

Verbose log only for this configuration block.

## auth_perdomain [yes/no]
## auth_domains ...

Override corresponding global directives.


# PAM MODULE

Implements authentication using libpam. Alternatively it can be configured to use
helper binary like extauth does. 

maddy should be built with CGo support enabled (default if C compiler is available) 
to use this module without 'use_helper' directive. 

```
smtp smtp://0.0.0.0 {
    auth pam { }
}
```

## debug [yes/no]

Verbose log only for this configuration block.

## use_helper

Use LibexecDirectory/maddy-pam-helper instead of directly calling libpam.
You need to use that if:
1. maddy is not compiled with CGo support, but maddy-pam-helper is built separately.
2. maddy is running as an unprivileged user and used PAM configuration requires additional 
privileges (e.g. when using system accounts).

For 2, you need to make maddy-pam-helper binary setuid, see 
cmd/maddy-pam-helper/README.md in source tree for details. 

TL;DR (assuming you have maddy group):
```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

# SHADOW MODULE

Implements authentication by reading /etc/shadow. Alternatively it can be 
configured to use helper binary like extauth does. 

```
smtp smtp://0.0.0.0 {
	auth shadow { }
}
```

## debug [yes/no]

Verbose log only for this configuration block.

## use_helper

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
