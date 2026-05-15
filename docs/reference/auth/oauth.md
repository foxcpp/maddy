# OAuth Bearer Token Authentication

`auth.oauth` implements OAuth Bearer Token authentication as defined
in [RFC 7628][rfc7628] and [RFC 6750][rfc6750].

It is not compatible with non-standard XOAUTH2 implementations, such as those
used by Google and Microsoft.

The provided token can be validated either by the server directly by decoding
JWT, or by making an introspection request ([RFC 7662][rfc7662]) to the
authorization server to validate the token and retrieve associated metadata.

## Configuration directives

```
auth.oauth [<url>] {
    [debug yes | no]
    [introspection auth | get | post | local]
    [introspection_url <url>]
    [http_header <key> <value>]
    [http_header <key> <value> ...]
    [introspection_timeout 5s]
    [scopes <scope...>]
    [username_attribute <attribute>]
    [active_attribute active]
    [active_value true]
    [jwt_key_id_template <template>]
    [jwt_key_table <table>]
    [jwt_valid_methods <method...>]
    [jwt_issuers <issuer...>]
    [jwt_expiry_leeway <duration>]
    [jwt_audience <table>]
}
```

### debug _yes|no_
Default: no

Enables debug logging.

---

### introspection _auth|get|post|local_
Default: auth

Defines the method used to validate the token. The following options are available:
- `auth`: Add token to the `Authorization` header and make a request to the introspection endpoint.
- `get`: Make a GET request to the introspection endpoint with the token appended to the URL.
- `post`: Make a POST request to the introspection endpoint with the token as `token` form-data field.
- `local`: Validate the token locally by decoding it as a JWT.

---

### introspection_url _<url>_
Default: (none)

The URL of the introspection endpoint to validate the token. Required if `introspection_method` is set
to `auth`, `get`, or `post`.

---

### http_header _<key> <value>_
Default: (none)

Additional HTTP headers to include in the introspection request. This can be used to provide server
credentials or other necessary information to the authorization server.

---

### introspection_timeout _<duration>_
Default: 5s

The timeout for the introspection request. If the request takes longer than this duration, authentication
will fail.

---

### scopes _<scope>_
Default: (none)

The required scope(s) for the token. If specified, the token must include all scopes to be
considered valid.

---

### username_attribute _<attribute>_
Default: email

The attribute in the token response that contains the username. This is used to set the username for the
authenticated user.

If attribute is a list of strings, the first non-empty value will be used as the username.

If client requests a specific username (e.g. via SASL authorization identity), it must match
the username extracted from the token for authentication to succeed. If attribute is a
list then the client requested username must match at least one of the values in the list.

---

### active_attribute _<attribute>_
Default: not specified

The attribute in the token response that indicates whether the token is active. This is used to determine if the
token is valid. If not specified, the token is considered active if the introspection request returns
a successful response.

---

### active_value _<value>_
Default: not specified

The value of the `active_attribute` that indicates the token is active. This is used to determine if the token is
valid. If not specified, the token is considered active if the `active_attribute` is present and has a truthy value
(not empty string, non-zero number, or boolean true).

---

### jwt_key_id_template _<template\>_
Default: `{kid}`

The template used to determine the key for `jwt_key_table` lookup. The template can include placeholders
for JWT header and body fields: azp, kid, alg. If azp or kid is missing from the token, the placeholder will be
replaced with `default`.

---

### jwt_key_table _<table\>_
Default: (none)

The table to use for looking up the key to validate JWT tokens. The lookup key is determined by applying
the `jwt_key_id_template` to the token's header and body fields.

Most `storage.blob` (including file and s3) can be used here as well as they
all also implement the `table` interface.

---

### jwt_valid_methods _<method\>_
Default: all supported methods.

The allowed signing algorithms for JWT tokens. If specified, the token's `alg` header field must match
one of these values to be considered valid.

It is not possible to enable `none` algorithm for JWT tokens.

---

### jwt_issuers _<issuer\>_
Default: (none)

If specified, the token's `iss` claim must match one of these values to be considered valid.

---

### jwt_expiry_leeway _<duration\>_
Default: 30s

The leeway to apply when validating the token's `exp` claim. This allows for some clock skew between the
token issuer and the server. The token is considered valid if the current time is
before `exp` + `jwt_expiry_leeway`.

---

### jwt_audience _<table\>_
Default: (none)

If specified, the token's `aud` claim must match one of the values in this table to be considered
valid.

[rfc7628]: https://tools.ietf.org/html/rfc7628
[rfc6750]: https://tools.ietf.org/html/rfc6750
[rfc7662]: https://tools.ietf.org/html/rfc7662
