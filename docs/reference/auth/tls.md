# TLS certificate authentication

`auth.tls` module implements TLS client certificate authentication for the server. It should be
used only if the server is correctly configured to use TLS while requiring client certificates.
If TLS is not used or client certificate is not provided by the client, `auth.tls` will
fail. Though it is possible to use classic username-password authentication as a fallback
by specifying multiple providers using `auth` directive multiple times.

Example:
```
smtp ... {
    tls {
        ...
        client_auth verify_if_given
        client_ca /path/to/ca.pem
    }
    auth tls
    auth pass_table ... # fallback for clients that do not support TLS client authentication

    ...
}
```

## Configuration directives

```
auth.tls {
    identity_fields san_email cn
    ignore_requested_identity no
    identity_normalize auto
    require_key_usage yes
    require_ext_key_usage yes
}
```

### identity_fields _field..._
Default: `san_email cn`

List of certificate fields to use when extracting client identity.

Valid values are: `san_email`, `cn`. SAN fields will use corresponding
fields of Subject Alternative Name extension, while `cn` will use Common Name field of
the certificate. PKCS#9 emailAddress field (commonly displayed as EMAILADDRESS or E in subject)
is obsolete and is not supported, SAN email field should be used instead.

If multiple fields are specified, they will be tried in specified order until a non-empty
value is found.  If no non-empty value is found, authentication will fail.

### ignore_requested_identity _yes|no_
Default: `no`

If set to `yes`, the server will ignore the identity requested by the client and will
always use the first non-empty value from `identity_fields` as the client identity. If set to
`no`, the server will use the identity requested by the client if it is present in the certificate
and is non-empty.

Most clients do not support SASL authorization identity and therefore cannot
request a specific identity to be used.

### identity_normalize _func_
Default: `auto`

Function used to normalize the extracted identity and requested identity. See
[Global configuration](../global-config) for details on available functions.

### require_key_usage _yes|no_
Default: `yes`

If set to `yes`, the server will require that the certificate has Key Usage extension with
Digital Signature bit set.

### require_ext_key_usage _yes|no_
Default: `yes`

If set to `yes`, the server will require that the certificate has Extended Key Usage extension
with Client Authentication bit set.
