# Using PAM authentication

maddy supports user authentication using PAM infrastructure via `auth.pam`
module.

In order to use it, however, either maddy itself should be compiled
with libpam support or a helper executable should be built and
installed into an appropriate directory.

It is recommended to use builtin libpam support if you are using
PAM as an intermediate for authentication provider not directly
supported by maddy.

If PAM authentication requires privileged access on the host system
(e.g. pam_unix.so aka /etc/shadow) then it is recommended to use
a privileged helper executable since maddy process itself won't
have access to it.

## Built-in PAM support

Binary artifacts provided for releases do not come with
libpam support. You should build maddy from source.

See [here](../building-from-source) for detailed instructions.

You should have libpam development files installed (`libpam-dev`
package on Ubuntu/Debian).

Then add `--tags 'libpam'` to the build command:
```
./build.sh --tags 'libpam'
```

Then you should be able to replace `local_authdb` implementation
in default configuration with `auth.pam`:
```
auth.pam local_authdb {
    use_helper no
}
```

## Helper executable

TL;DR
```
git clone https://github.com/foxcpp/maddy
cd maddy/cmd/maddy-pam-helper
gcc pam.c main.c -lpam -o maddy-pam-helper
```

Copy the resulting executable into /usr/lib/maddy/ and make
it setuid-root so it can read /etc/shadow (if that's necessary):
```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

Then you should be able to replace `local_authdb` implementation
in default configuration with `auth.pam`:
```
auth.pam local_authdb {
    use_helper yes
}
```

## Account names

Since PAM does not use emails for authentication you should also
configure storage backend to use username only as an account identifier,
not full email addresses:
```
storage.imapsql local_mailboxes {
    ...
    delivery_map email_localpart
    auth_normalize precis_casefold
}
```

This way, when authenticating as `foxcpp`, it will be mapped to
`foxcpp` storage account. E.g. you will need to run
`maddy imap-accts create foxcpp`, without the domain part.

If you have existing accounts, you will need to rename them.

Change to `auth_normalize` is necessary so that normalization function
will not attempt to parse authentication identity as a email.

When a email is received, `delivery_map email_localpart` will strip
the domain part before looking up the account. That is,
`foxcpp@example.org` will be become just `foxcpp`.

You also need to make `authorize_sender` check (used in `submission` endpoint)
accept non-email usernames:
```
authorize_sender {
  ...
  auth_normalize precis_casefold
  user_to_email regexp "(.*)" "$1@$(primary_domain)"
}
```
Note that is would work only if clients use only one domain as sender (`$(primary_domain)`).
If you want to allow sending from all domains, you need to remove `authorize_sender` check
altogether since it is not currently supported.

## PAM service

You should create a PAM configuration file for maddy to use.
Place it into /etc/pam.d/maddy.
Here is the minimal example using pam_unix (shadow database).
```
#%PAM-1.0
auth	required	pam_unix.so
account	required	pam_unix.so
```

Here is the configuration example you could use on Ubuntu
to use the authentication config system itself uses:
```
#%PAM-1.0

@include common-auth
@include common-account
@include common-session
```
