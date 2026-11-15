# Dovecot SASL endpoint

The `dovecot_sasld` module provides a Dovecot-compatible SASL authentication
listener. It lets an SMTP server that supports the Dovecot authentication
protocol use maddy's configured authentication providers.

This endpoint offers password-based SASL mechanisms that send credentials in
cleartext, without encryption. Use a Unix socket accessible only to trusted
local processes, or protect a TCP connection with a secure transport outside
this endpoint. The endpoint does not provide a `tls` directive.

```
dovecot_sasld unix:/run/maddy/auth-client.sock {
    auth &local_authdb
    sasl_login no
}
```

`&local_authdb` is an example reference to a separately configured
authentication module that supports password authentication. The endpoint
accepts listener addresses in the standard endpoint format; for example,
`tcp://127.0.0.1:12345` for TCP or `unix:/run/maddy/auth-client.sock` for a
Unix socket.

## Configuration directives

### auth _module-reference_

Add a authentication provider.

Supported authentication mechanisms: SASL PLAIN

---

### sasl_login _boolean_
Default: `no`

Enable the SASL LOGIN mechanism. LOGIN is not offered by default and, like
PLAIN, is available only when a compatible authentication provider is
configured.

---

### auth_map _module-reference_
Default: global `auth_map` value (`identity` if not set globally)

Use the specified table to translate usernames before they are passed to the
authentication provider. Username normalization is controlled by
`auth_map_normalize`. See [global configuration](/reference/global-config)
for details.

---

### auth_map_normalize _function_
Default: global `auth_map_normalize` value (`auto` if not set globally)

Choose the normalization function applied to usernames before `auth_map` is
used. See [global configuration](/reference/global-config#auth_map_normalize)
for the available functions.
