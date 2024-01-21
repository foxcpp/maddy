# PAM

auth.pam module implements authentication using libpam. Alternatively it can be configured to
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

### debug _boolean_ 
Default: `no`

Enable verbose logging for all modules. You don't need that unless you are
reporting a bug.

---

### use_helper _boolean_
Default: `no`

Use `LibexecDirectory/maddy-pam-helper` instead of directly calling libpam.
You need to use that if:

1. maddy is not compiled with libpam, but `maddy-pam-helper` is built separately.
2. maddy is running as an unprivileged user and used PAM configuration requires additional privileges (e.g. when using system accounts).

For 2, you need to make `maddy-pam-helper` binary setuid, see
README.md in source tree for details.

TL;DR (assuming you have the maddy group):

```
chown root:maddy /usr/lib/maddy/maddy-pam-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-pam-helper
```

