# TLS configuration

## Server-side

TLS certificates are obtained by modules called "certificate loaders". 'tls' directive
arguments specify name of loader to use and arguments. Due to syntax limitations
advanced configuration for loader should be specified using 'loader' directive, see
below.

```
tls file cert.pem key.pem {
	protocols tls1.2 tls1.3
	curves X25519
	ciphers ...
}

tls {
	loader file cert.pem key.pem {
		# Options for loader go here.
	}
	protocols tls1.2 tls1.3
	curves X25519
	ciphers ...
}
```

### Available certificate loaders

- `file` – Accepts argument pairs specifying certificate and then key.
  E.g. `tls file certA.pem keyA.pem certB.pem keyB.pem`.
  If multiple certificates are listed, SNI will be used.
- `acme` – Automatically obtains a certificate using ACME protocol (Let's Encrypt)
- `off` – Not really a loader but a special value for tls directive, 
  explicitly  disables TLS for endpoint(s).

## Advanced TLS configuration

**Note: maddy uses secure defaults and TLS handshake is resistant to active downgrade attacks. There is no need to change anything in most cases.**

---

### protocols _min-version_ _max-version_ | _version_
Default: `tls1.0 tls1.3`

Minimum/maximum accepted TLS version. If only one value is specified, it will
be the only one usable version.

Valid values are: `tls1.0`, `tls1.1`, `tls1.2`, `tls1.3`

---

### ciphers _ciphers..._ 
Default: Go version-defined set of 'secure ciphers', ordered by hardware
performance

List of supported cipher suites, in preference order. Not used with TLS 1.3.

Valid values:

- `RSA-WITH-RC4128-SHA`
- `RSA-WITH-3DES-EDE-CBC-SHA`
- `RSA-WITH-AES128-CBC-SHA`
- `RSA-WITH-AES256-CBC-SHA`
- `RSA-WITH-AES128-CBC-SHA256`
- `RSA-WITH-AES128-GCM-SHA256`
- `RSA-WITH-AES256-GCM-SHA384`
- `ECDHE-ECDSA-WITH-RC4128-SHA`
- `ECDHE-ECDSA-WITH-AES128-CBC-SHA`
- `ECDHE-ECDSA-WITH-AES256-CBC-SHA`
- `ECDHE-RSA-WITH-RC4128-SHA`
- `ECDHE-RSA-WITH-3DES-EDE-CBC-SHA`
- `ECDHE-RSA-WITH-AES128-CBC-SHA`
- `ECDHE-RSA-WITH-AES256-CBC-SHA`
- `ECDHE-ECDSA-WITH-AES128-CBC-SHA256`
- `ECDHE-RSA-WITH-AES128-CBC-SHA256`
- `ECDHE-RSA-WITH-AES128-GCM-SHA256`
- `ECDHE-ECDSA-WITH-AES128-GCM-SHA256`
- `ECDHE-RSA-WITH-AES256-GCM-SHA384`
- `ECDHE-ECDSA-WITH-AES256-GCM-SHA384`
- `ECDHE-RSA-WITH-CHACHA20-POLY1305`
- `ECDHE-ECDSA-WITH-CHACHA20-POLY1305`

---

### curves _curves..._
Default: defined by Go version

The elliptic curves that will be used in an ECDHE handshake, in preference
order.

Valid values: `p256`, `p384`, `p521`, `X25519`.

## Client

`tls_client` directive allows to customize behavior of TLS client implementation,
notably adjusting minimal and maximal TLS versions and allowed cipher suites,
enabling TLS client authentication.

```
tls_client {
    protocols tls1.2 tls1.3
    ciphers ...
    curves X25519
    root_ca /etc/ssl/cert.pem

    cert /etc/ssl/private/maddy-client.pem
    key /etc/ssl/private/maddy-client.pem
}
```

---

###  protocols _min-version_ _max-version_ | _version_
Default: `tls1.0 tls1.3`

Minimum/maximum accepted TLS version. If only one value is specified, it will
be the only one usable version.

Valid values are: `tls1.0`, `tls1.1`, `tls1.2`, `tls1.3`

---

### ciphers _ciphers..._
Default: Go version-defined set of 'secure ciphers', ordered by hardware
performance

List of supported cipher suites, in preference order. Not used with TLS 1.3.

See TLS server configuration for list of supported values.

---

### curves _curves..._
Default: defined by Go version

The elliptic curves that will be used in an ECDHE handshake, in preference
order.

Valid values: `p256`, `p384`, `p521`, `X25519`.

---

### root_ca _paths..._
Default: system CA pool

List of files with PEM-encoded CA certificates to use when verifying
server certificates.

---

###  cert _cert-path_ <br> key _key-path_
Default: not specified

Present the specified certificate when server requests a client certificate.
Files should use PEM format. Both directives should be specified.
