# /etc/shadow

auth.shadow module implements authentication by reading /etc/shadow. Alternatively it can be
configured to use helper binary like auth.external does.

```
auth.shadow {
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

Use `LibexecDirectory/maddy-shadow-helper` instead of directly reading `/etc/shadow`.
You need to use that if maddy is running as an unprivileged user
privileges (e.g. when using system accounts).

You need to make `maddy-shadow-helper` binary setuid, see
cmd/maddy-shadow-helper/README.md in source tree for details.

TL;DR (assuming you have maddy group):

```
chown root:maddy /usr/lib/maddy/maddy-shadow-helper
chmod u+xs,g+x,o-x /usr/lib/maddy/maddy-shadow-helper
```

