# Automatic certificate management via ACME

Maddy supports obtaining certificates using ACME protocol.

To use it, create a configuration name for `tls.loader.acme`
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

You can also use a global `tls` directive to use automatically
obtained certificates for all endpoints:

```
tls {
    loader acme {
        email maddy-acme@example.org
        agreed
        challenge dns-01
    }
}
```

Note: `tls &local_tls` as a global directive won't work because
global directives are initialized before other configuration blocks.

Currently the only supported challenge is `dns-01` one therefore
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

### debug _boolean_
Default: global directive value

Enable debug logging.

---

### hostname _str_
**Required.**<br>
Default: global directive value

Domain name to issue certificate for.

---

### store_path _path_
Default: `state_dir/acme`

Where to store issued certificates and associated metadata.
Currently only filesystem-based store is supported.

---

### ca _url_
Default: Let's Encrypt production CA

URL of ACME directory to use.

---

### test_ca _url_
Default: Let's Encrypt staging CA

URL of ACME directory to use for retries should
primary CA fail.

maddy will keep attempting to issues certificates
using `test_ca` until it succeeds then it will switch
back to the one configured via 'ca' option.

This avoids rate limit issues with production CA.

---

### override_domain _domain_
Default: not set

Override the domain to set the TXT record on for DNS-01 challenge.
This is to delegate the challenge to a different domain.

See https://www.eff.org/deeplinks/2018/02/technical-deep-dive-securing-automation-acme-dns-challenge-validation
for explanation why this might be useful.

---

### email _str_
Default: not set

Email to pass while registering an ACME account.

---

### agreed _boolean_
Default: false

Whether you agreed to ToS of the CA service you are using.

---

### challenge `dns-01`
Default: not set

Challenge(s) to use while performing domain verification.

## DNS providers

Support for some providers is not provided by standard builds.
To be able to use these, you need to compile maddy
with "libdns_PROVIDER" build tag.
E.g.
```
./build.sh --tags 'libdns_googleclouddns'
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

See [https://github.com/libdns/cloudflare#authenticating](https://github.com/libdns/cloudflare#authenticating)

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

- rfc2136 (non-default)

```
dns rfc2136 {
    key_name "..."
    # Secret
    key "..."
    # HMAC algorithm used to generate the key, lowercase, e.g. hmac-sha512
    key_alg "..."
    # server to which the dynamic update will be sent, e.g. 127.0.0.1
    # you can also specify the port: 127.0.0.1:53
    server "..."
}
```

- acmedns (non-default)

```
dns acmedns {
    username "..."
    password "..."
    subdomain "..."
    server_url "..."
}
```
