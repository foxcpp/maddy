# Installation & initial configuration

This is the practical guide on how to set up a mail server using maddy for
personal use. It omits most of the technical details for brevity and just gives
you the minimal list of things you need to be aware of and what to do to make
stuff work.

For purposes of clarity, these values are used in this tutorial as examples,
wherever you see them, you need to replace them with your actual values:

- Domain: example.org
- MX domain (hostname): mx.example.org
- IPv4 address: 10.2.3.4
- IPv6 address: 2001:beef::1

## Getting a server

Where to get a server to run maddy on is out of the scope of this article. Any
VPS (virtual private server) will work fine for small configurations. However,
there are a few things to keep in mind:

- Make sure your provider does not block SMTP traffic (25 TCP port). Most VPS
  providers don't do it, but some "cloud" providers (such as Google Cloud) do
  it, so you can't host your mail there.
- ...

## Installing maddy

Your options are:

* Pre-built tarball (Linux, amd64)

    Available on [GitHub](https://github.com/foxcpp/maddy/releases) or
    [foxcpp.dev](https://foxcpp.dev/maddy-builds/).

    The tarball contains the full filesystem tree so you can directly
    extract it into root directory (or you can take just executable).

* Docker image (Linux, amd64)

    ```
    docker pull foxcpp/maddy:latest
    ```

    See README at [hub.docker.com](https://hub.docker.com/r/foxcpp/maddy)
    for Docker-specific instructions.

* Building from source

    See [here](../building-from-source) for instructions.

## Host name + domain

Open /etc/maddy/maddy.conf with vim^W your favorite editor and change
the following lines to match your server name and domain you want to handle
mail for.

```
$(hostname) = mx.example.org
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
$ sudo setfacl -R -m u:maddy:rX /etc/ssl/example.org.crt /etc/ssl/example.org.key
```

### Let's Encrypt and certbot

If you use certbot to manage your certificates, you can simply symlink
/etc/maddy/certs into /etc/letsencrypt/live. maddy will pick the right
certificate depending on the domain you specified during installation.

You still need to make keys readable for maddy, though:
```
$ sudo setfacl -R -m u:maddy:rX /etc/letsencrypt/{live,archive}
```

maddy reloads TLS certificates from disk once in a minute so it will notice
renewal. It is possible to force reload via `systemctl reload maddy` (or just
`killall -USR2 maddy`).

## First run

```
systemctl daemon-reload
systemctl start maddy
```

Well, it should be running now, except that it is useless because we haven't
configured DNS records.

## DNS records

How it is configured depends on your DNS provider (or server, if you run your
own). Here is how your DNS zone should look like:
```
; Basic domain->IP records, you probably already have them.
example.org.   A     10.2.3.4
example.org.   AAAA  2001:beef::1

; It says that "server example.org is handling messages for example.org".
example.org.   MX    10 example.org.

; Use SPF to say that the servers in "MX" above are allowed to send email
; for this domain, and nobody else.
example.org.   TXT   "v=spf1 mx -all"

; Opt-in into DMARC with permissive policy and request reports about broken
; messages.
_dmarc.example.org.   TXT    "v=DMARC1; p=none; ruf=postmaster@example.org"
```

And the last one, DKIM key, is a bit tricky. maddy generated a key for you on
the first start-up. You can find it in
/var/lib/maddy/dkim_keys/example.org_default.dns. You need to put it in a TXT
record for `default._domainkey.example.org` domain, like that:
```
default._domainkey.example.org    TXT   "v=DKIM1; k=ed25519; p=nAcUUozPlhc4VPhp7hZl+owES7j7OlEv0laaDEDBAqg="
```

## MTA-STS

By default SMTP is not protected against active attacks. MTA-STS policy tells
compatible senders to always use properly authenticated TLS when talking to
your server, offering a simple-to-deploy way to protect your server against
MitM attacks on port 25.

Basically, you to create a file with following contents and make it available
at https://mta-sts.example.org/.well-known/mta-sts.txt:
```
mode: enforce
max_age: 604800
mx: mx.example.org
```

**Note**: mx.example.org in the file is your MX hostname, example.org in URL is
the domain you are receiving messages for. In simple configurations, they are
may be the same, but this is not the case for more complex setups.
If you have multiple MX servers - add them all once per line, like that:
```
mx: mx1.example.org
mx: mx2.example.org
```

## User accounts and maddyctl

A mail server is useless without mailboxes, right? Unlike software like postfix
and dovecot, maddy uses "virtual users" by default, meaning it does not care or
know about system users.

IMAP mailboxes ("accounts") and authentication credentials are kept separate.

To register user credentials, use `maddyctl creds create` command.
Like that:
```
$ maddyctl creds create postmaster@example.org
```

Note the username is a e-mail address. This is required as username is used to
authorize IMAP and SMTP access (unless you configure custom mappings, not
described here).

After registering the user credentials, you also need to create a local
storage account:
```
$ maddyctl imap-acct create postmaster@example.org
```

That is it. Now you have your first e-mail address. when authenticating using
your e-mail client, do not forget the username is "postmaster@example.org", not
just "postmaster".

You may find running `maddyctl creds --help` and `maddyctl imap-acct --help`
useful to learn about other commands. Note that IMAP accounts and credentials
are managed separately yet usernames should match by default for things to
work.

## Optional: Install and use fail2ban

The email world is full of annoying bots just like Web (these annoying scanners
looking for PhpMyAdmin on your blog). fail2ban can help you get rid of them by
temporary blocking offending IPs.

1. Install the fail2ban package and the python systemd module using your distribution package manager
For Debian-based distributions:
```
apt-get install fail2ban python3-systemd
```

2. build.sh already installed necessary jail configuration files, but you have to
   enable them. Open /etc/fail2ban/jail.d/common.local (create one if needed)
   and add the following lines:
```
[maddy-auth]
enabled = true

[maddy-dictonary-attack]
enabled = true
```

Now start or restart the fail2ban daemon:
```
systemctl restart fail2ban
```

Keep in mind that the maddy jail configuration uses a different much longer
bantime value. This means users will get IP-banned for quite a long time (4
days) after 5 failed login attempts. You might want to change that to a smaller
period by editing /etc/fail2ban/jail.d/common.local:
```
[maddy-auth]
bantime = 1h
```
