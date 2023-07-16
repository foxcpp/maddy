# Native NetAuth

maddy supports authentication via NetAuth using direct entity
authentication checks.  Passwords are verified by the NetAuth server.

maddy needs to know the Entity ID to use for authentication.  It must
match the string the user provides for the Local Atom part of their
mail address.

Note that storage backends conventionally use email addresses.  Since
NetAuth recommends *nix compatible usernames, you will need to map the
email identifiers to NetAuth Entity IDs using `auth_map` (see
documentation page for used storage backend).

auth.netauth also can be used as a table module.  This way you can
check whether the account exists.

Note that the configuration fragment provided below is very sparse.
This is because NetAuth expects to read most of its common
configuration values from the system NetAuth config file located at
`/etc/netauth/config.toml`.

```
auth.netauth {
  require_group "maddy-users"
  debug off
}
```

```
auth.netauth {}
```

## Configuration directives

### require_group _group_

Optional.

Group that entities must possess to be able to use maddy services.
This can be used to provide email to just a subset of the entities
present in NetAuth.

---

### debug `on` | `off`

Default: `off`
