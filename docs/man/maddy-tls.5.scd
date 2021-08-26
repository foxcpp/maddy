maddy-tls(5) "maddy mail server" "maddy reference documentation"

; TITLE Advanced TLS configuration

# TLS server configuration

TLS certificates are obtained by modules called "certificate loaders". 'tls' directive
arguments specify name of loader to use and arguments. Due to syntax limitations
advanced configuration for loader should be specified using 'loader' directive, see
below.

```
tls file cert.pem key.pem {
	protocols tls1.2 tls1.3
	curve X25519
	ciphers ...
}

tls {
	loader file cert.pem key.pem {
		# Options for loader go here.
	}
	protocols tls1.2 tls1.3
	curve X25519
	ciphers ...
}
```

## Available certificate loaders

- file

	Accepts argument pairs specifying certificate and then key.
	E.g. 'tls file certA.pem keyA.pem certB.pem keyB.pem'

	If multiple certificates are listed, SNI will be used.

- acme

	Automatically obtains a certificate using ACME protocol (Let's Encrypt)

	See below for details.

- off

	Not really a loader but a special value for tls directive, explicitly disables TLS for
	endpoint(s).

## Advanced TLS configuration

*Note: maddy uses secure defaults and TLS handshake is resistant to active downgrade attacks.*
*There is no need to change anything in most cases.*

*Syntax*: ++
    protocols _min_version_ _max_version_ ++
    protocols _version_ ++
*Default*: tls1.0 tls1.3

Minimum/maximum accepted TLS version. If only one value is specified, it will
be the only one usable version.

Valid values are: tls1.0, tls1.1, tls1.2, tls1.3

*Syntax*: ciphers _ciphers..._ ++
*Default*: Go version-defined set of 'secure ciphers', ordered by hardware
performance

List of supported cipher suites, in preference order. Not used with TLS 1.3.

Valid values:

- RSA-WITH-RC4128-SHA
- RSA-WITH-3DES-EDE-CBC-SHA
- RSA-WITH-AES128-CBC-SHA
- RSA-WITH-AES256-CBC-SHA
- RSA-WITH-AES128-CBC-SHA256
- RSA-WITH-AES128-GCM-SHA256
- RSA-WITH-AES256-GCM-SHA384
- ECDHE-ECDSA-WITH-RC4128-SHA
- ECDHE-ECDSA-WITH-AES128-CBC-SHA
- ECDHE-ECDSA-WITH-AES256-CBC-SHA
- ECDHE-RSA-WITH-RC4128-SHA
- ECDHE-RSA-WITH-3DES-EDE-CBC-SHA
- ECDHE-RSA-WITH-AES128-CBC-SHA
- ECDHE-RSA-WITH-AES256-CBC-SHA
- ECDHE-ECDSA-WITH-AES128-CBC-SHA256
- ECDHE-RSA-WITH-AES128-CBC-SHA256
- ECDHE-RSA-WITH-AES128-GCM-SHA256
- ECDHE-ECDSA-WITH-AES128-GCM-SHA256
- ECDHE-RSA-WITH-AES256-GCM-SHA384
- ECDHE-ECDSA-WITH-AES256-GCM-SHA384
- ECDHE-RSA-WITH-CHACHA20-POLY1305
- ECDHE-ECDSA-WITH-CHACHA20-POLY1305

*Syntax*: curve _curves..._ ++
*Default*: defined by Go version

The elliptic curves that will be used in an ECDHE handshake, in preference
order.

Valid values: p256, p384, p521, X25519.

# TLS client configuration

tls_client directive allows to customize behavior of TLS client implementation,
notably adjusting minimal and maximal TLS versions and allowed cipher suites,
enabling TLS client authentication.

```
tls_client {
    protocols tls1.2 tls1.3
    ciphers ...
    curve X25519
    root_ca /etc/ssl/cert.pem

    cert /etc/ssl/private/maddy-client.pem
    key /etc/ssl/private/maddy-client.pem
}
```

*Syntax*: ++
    protocols _min_version_ _max_version_ ++
    protocols _version_ ++
*Default*: tls1.0 tls1.3

Minimum/maximum accepted TLS version. If only one value is specified, it will
be the only one usable version.

Valid values are: tls1.0, tls1.1, tls1.2, tls1.3

*Syntax*: ciphers _ciphers..._ ++
*Default*: Go version-defined set of 'secure ciphers', ordered by hardware
performance

List of supported cipher suites, in preference order. Not used with TLS 1.3.

See TLS server configuration for list of supported values.

*Syntax*: curve _curves..._ ++
*Default*: defined by Go version

The elliptic curves that will be used in an ECDHE handshake, in preference
order.

Valid values: p256, p384, p521, X25519.

*Syntax*: root_ca _paths..._ ++
*Default*: system CA pool

List of files with PEM-encoded CA certificates to use when verifying
server certificates.

*Syntax*: ++
    cert _cert_path_ ++
    key _key_path_ ++
*Default*: not specified

Present the specified certificate when server requests a client certificate.
Files should use PEM format. Both directives should be specified.

# Automatic certificate management via ACME

```
tls.loader.acme {
    debug off
    hostname example.maddy.invalid
    store_path /var/lib/maddy/acme
    ca https://acme-v02.api.letsencrypt.org/directory
    test_ca https://acme-staging-v02.api.letsencrypt.org/directory
    email test@maddy.invalid
    agreed off
    challenge dns-01
    dns ...
}
```

Maddy supports obtaining certificates using ACME protocol.

To use it, create a configuration name for tls.loader.acme
and reference it from endpoints that should use automatically
configured certificates:
```
tls.loader.acme local_tls {
    email put-your-email-here@example.org
    agreed # indicate your agreement with Let's Encrypt ToS
    challenge dns-01
}

smtp tcp://127.0.0.1:25 {
    tls &local_tls
    ...
}
```

Currently the only supported challenge is dns-01 one therefore
you also need to configure the DNS provider:
```
tls.loader.acme local_tls {
    email maddy-acme@example.org
    agreed
    challenge dns-01
    dns PROVIDER_NAME {
        ...
    }
}
```
See below for supported providers and necessary configuration
for each.

## Configuration directives

*Syntax:* debug _boolean_ ++
*Default:* global directive value

Enable debug logging.

*Syntax:* hostname _str_ ++
*Default:* global directive value

Domain name to issue certificate for. Required.

*Syntax:* store_path _path_ ++
*Default:* state_dir/acme

Where to store issued certificates and associated metadata.
Currently only filesystem-based store is supported.

*Syntax:* ca _url_ ++
*Default:* Let's Encrypt production CA

URL of ACME directory to use.

*Syntax:* test_ca _url_ ++
*Default:* Let's Encrypt staging CA

URL of ACME directory to use for retries should
primary CA fail.

maddy will keep attempting to issues certificates
using test_ca until it succeeds then it will switch
back to the one configured via 'ca' option.

This avoids rate limit issues with production CA.

*Syntax:* email _str_ ++
*Default:* not set

Email to pass while registering an ACME account.

*Syntax:* agreed _boolean_ ++
*Default:* false

Whether you agreed to ToS of the CA service you are using.

*Syntax:* challenge dns-01 ++
*Default:* not set

Challenge(s) to use while performing domain verification.

## DNS providers

Support for some providers is not provided by standard builds.
To be able to use these, you need to compile maddy
with "libdns_PROVIDER" build tag.
E.g.
```
./build.sh -tags 'libdns_googleclouddns'
```

- gandi

```
dns gandi {
    api_token "token"
}
```

- digitalocean

```
dns digitalocean {
    api_token "..."
}
```

- cloudflare

See https://github.com/libdns/cloudflare#authenticating

```
dns cloudflare {
    api_token "..."
}
```

- vultr

```
dns vultr {
    api_token "..."
}
```

- hetzner

```
dns hetzner {
    api_token "..."
}
```

- namecheap

```
dns namecheap {
    api_key "..."
    api_username "..."

    # optional: API endpoint, production one is used if not set.
    endpoint "https://api.namecheap.com/xml.response"

    # optional: your public IP, discovered using icanhazip.com if not set
    client_ip 1.2.3.4
}
```

- googleclouddns (non-default)

```
dns googleclouddns {
    project "project_id"
    service_account_json "path"
}
```

- route53 (non-default)

```
dns route53 {
    secret_access_key "..."
    access_key_id "..."
    # or use environment variables: AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY
}
```

- leaseweb (non-default)

```
dns leaseweb {
    api_key "key"
}
```

- metaname (non-default)

```
dns metaname {
    api_key "key"
    account_ref "reference"
}
```

- alidns (non-default)

```
dns alidns {
    key_id "..."
    key_secret "..."
}
```

- namedotcom (non-default)

```
dns namedotcom {
    user "..."
    token "..."
}
```
