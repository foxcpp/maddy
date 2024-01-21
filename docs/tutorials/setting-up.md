# Installation & initial configuration

This is the practical guide on how to set up a mail server using maddy for
personal use. It omits most of the technical details for brevity and just gives
you the minimal list of things you need to be aware of and what to do to make
stuff work.

For purposes of clarity, these values are used in this tutorial as examples,
wherever you see them, you need to replace them with your actual values:

- Domain: example.org
- MX domain (hostname): mx1.example.org
- IPv4 address: 10.2.3.4
- IPv6 address: 2001:beef::1

## Getting a server

Where to get a server to run maddy on is out of the scope of this article. Any
VPS (virtual private server) will work fine for small configurations. However,
there are a few things to keep in mind:

- Make sure your provider does not block SMTP traffic (25 TCP port). Most VPS
  providers don't do it, but some "cloud" providers (such as Google Cloud) do
  it, so you can't host your mail there.

- It is recommended to run your own DNS resolver with DNSSEC verification
  enabled.

## Installing maddy

Your options are:

* Pre-built tarball (Linux, amd64)

    Available on [GitHub](https://github.com/foxcpp/maddy/releases) or
    [maddy.email/builds](https://maddy.email/builds/).

	The tarball includes maddy executable you can
	copy into /usr/local/bin as well as systemd unit file you can
	use on systemd-based distributions for automatic startup and service
	supervision. You should also create "maddy" user and group.
	See below for more detailed instructions.

* Docker image (Linux, amd64)

    ```
    docker pull foxcpp/maddy:0.6
    ```

    See [here](../../docker) for Docker-specific instructions.

* Building from source

    See [here](../building-from-source) for instructions.

* Arch Linux packages

	For Arch Linux users, `maddy` and `maddy-git` PKGBUILDs are available
	in AUR. Additionally, binary packages are available in 3rd-party
	repository at [https://maddy.email/archlinux/](https://maddy.email/archlinux/)

## System configuration (systemd-based distribution)

If you built maddy from source and used `./build.sh install` then
systemd unit files should be already installed. If you used
a pre-built tarball - copy `systemd/*.service` to `/etc/systemd/system`
manually.

You need to reload service manager configuration to make service available:

```
systemctl daemon-reload
```

Additionally, you should create maddy user and group. Unlike most other
Linux mail servers, maddy never runs as root.

```
useradd -mrU -s /sbin/nologin -d /var/lib/maddy -c "maddy mail server" maddy
```

## Host name + domain

Open /etc/maddy/maddy.conf with vim^W your favorite editor and change
the following lines to match your server name and domain you want to handle
mail for.
If you setup a very small mail server you can use example.org in both fields.
However, to easier a future migration of service, it's recommended to use a
separate DNS entry for that purpose. It's usually mx1.example.org, mx2, etc.
You can of course use another subdomain, for instance: smtp1.example.org.
An email failover server will become possible if you forward mx2.example.org
to another server (as long as you configure it to handle your domain).

```
$(hostname) = mx1.example.org
$(primary_domain) = example.org
```

If you want to handle multiple domains, you still need to designate
one as "primary". Add all other domains to the `local_domains` line:

```
$(local_domains) = $(primary_domain) example.com other.example.com
```

## TLS certificates

One thing that can't be automagically configured is TLS certs. If you already
have them somewhere - use them, open /etc/maddy/maddy.conf and put the right
paths in. You need to make sure maddy can read them while running as
unprivileged user (maddy never runs as root, even during start-up), one way to
do so is to use ACLs (replace with your actual paths):
```
$ sudo setfacl -R -m u:maddy:rX /etc/ssl/mx1.example.org.crt /etc/ssl/mx1.example.org.key
```

maddy reloads TLS certificates from disk once in a minute so it will notice
renewal. It is possible to force reload via `systemctl reload maddy` (or just
`killall -USR2 maddy`).

### Let's Encrypt and certbot

If you use certbot to manage your certificates, you can simply symlink
/etc/maddy/certs into /etc/letsencrypt/live. maddy will pick the right
certificate depending on the domain you specified during installation.

You still need to make keys readable for maddy, though:
```
$ sudo setfacl -R -m u:maddy:rX /etc/letsencrypt/{live,archive}
```

### ACME.sh

If you use acme.sh to manage your certificates, you could simply run:

```
mkdir -p /etc/maddy/certs/mx1.example.org
acme.sh --force --install-cert -d mx1.example.org \
  --key-file       /etc/maddy/certs/mx1.example.org/privkey.pem  \
  --fullchain-file /etc/maddy/certs/mx1.example.org/fullchain.pem
```

## First run

```
systemctl start maddy
```

The daemon should be running now, except that it is useless because we haven't
configured DNS records.

## DNS records

How it is configured depends on your DNS provider (or server, if you run your
own). Here is how your DNS zone should look like:
```
; Basic domain->IP records, you probably already have them.
example.org.   A     10.2.3.4
example.org.   AAAA  2001:beef::1

; It says that "server mx1.example.org is handling messages for example.org".
example.org.   MX    10 mx1.example.org.
; Of course, mx1 should have A/AAAA entry as well:
mx1.example.org.   A     10.2.3.4
mx1.example.org.   AAAA  2001:beef::1

; Use SPF to say that the servers in "MX" above are allowed to send email
; for this domain, and nobody else.
example.org.     TXT   "v=spf1 mx ~all"
; It is recommended to server SPF record for both domain and MX hostname
mx1.example.org. TXT   "v=spf1 a ~all"

; Opt-in into DMARC with permissive policy and request reports about broken
; messages.
_dmarc.example.org.   TXT    "v=DMARC1; p=quarantine; ruf=mailto:postmaster@example.org"

; Mark domain as MTA-STS compatible (see the next section)
; and request reports about failures to be sent to postmaster@example.org
_mta-sts.example.org.   TXT    "v=STSv1; id=1"
_smtp._tls.example.org. TXT    "v=TLSRPTv1;rua=mailto:postmaster@example.org"
```

And the last one, DKIM key, is a bit tricky. maddy generated a key for you on
the first start-up. You can find it in
/var/lib/maddy/dkim_keys/example.org_default.dns. You need to put it in a TXT
record for `default._domainkey.example.org.` domain, like that:
```
default._domainkey.example.org.    TXT   "v=DKIM1; k=ed25519; p=nAcUUozPlhc4VPhp7hZl+owES7j7OlEv0laaDEDBAqg="
```

## MTA-STS and DANE

By default SMTP is not protected against active attacks. MTA-STS policy tells
compatible senders to always use properly authenticated TLS when talking to
your server, offering a simple-to-deploy way to protect your server against
MitM attacks on port 25.

Basically, you to create a file with following contents and make it available
at https://mta-sts.example.org/.well-known/mta-sts.txt:
```
version: STSv1
mode: enforce
max_age: 604800
mx: mx1.example.org
```

**Note**: mx1.example.org in the file is your MX hostname, In a simple configuration,
it will be the same as your hostname example.org.
In a more complex setups, you would have multiple MX servers - add them all once
per line, like that:

```
mx: mx1.example.org
mx: mx2.example.org
```

It is also recommended to set a TLSA (DANE) record.
Use https://www.huque.com/bin/gen_tlsa to generate one.
Set port to 25, Transport Protocol to "tcp" and Domain Name to **the MX hostname**.
Example of a valid record:
```
_25._tcp.mx1.example.org. TLSA 3 1 1 7f59d873a70e224b184c95a4eb54caa9621e47d48b4a25d312d83d96e3498238
```

## User accounts and maddy command

A mail server is useless without mailboxes, right? Unlike software like postfix
and dovecot, maddy uses "virtual users" by default, meaning it does not care or
know about system users.

IMAP mailboxes ("accounts") and authentication credentials are kept separate.

To register user credentials, use `maddy creds create` command.
Like that:
```
$ maddy creds create postmaster@example.org
```

Note the username is a e-mail address. This is required as username is used to
authorize IMAP and SMTP access (unless you configure custom mappings, not
described here).

After registering the user credentials, you also need to create a local
storage account:
```
$ maddy imap-acct create postmaster@example.org
```

Note: to run `maddy` CLI commands, your user should be in the `maddy`
group. Alternatively, just use `sudo -u maddy`.

That is it. Now you have your first e-mail address. when authenticating using
your e-mail client, do not forget the username is "postmaster@example.org", not
just "postmaster".

You may find running `maddy creds --help` and `maddy imap-acct --help`
useful to learn about other commands. Note that IMAP accounts and credentials
are managed separately yet usernames should match by default for things to
work.
