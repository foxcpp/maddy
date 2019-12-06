# Installation & initial configuration

This is the practical guide on how to set up a mail server using maddy for
personal use. It omits most of the technical details for brevity and just gives
you the minimal list of things you need to be aware of and what to do to make
stuff work.

For purposes of clarity, these values are used in this tutorial as examples,
wherever you see them, you need to replace them with your actual values:

- Domain: example.org
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

Since there are currently no pre-compiled binaries for maddy, we are going to
build it from the source. Nothing scary, this is relatively easy to do with Go.

One dependency you need to figure out is the C compiler, it is needed for
SQLite3 support which is used in the default configuration. On Debian-based
distributions, this should be enough:
```
# apt-get install build-essential
```

get.sh script will do the rest for you:

```
$ curl 'https://foxcpp.dev/maddy/get.sh' | bash
```

It will leave `maddy-setup` directory lying around, you might want to keep it
so you don't have to redownload and recompile everything on update.

*Note:* If you can't / don't use this script for some reason, instructions for
manual installation can be found
[here](../manual-installation)

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

Additionally, it is a good idea to automatically restart
maddy on certificate renewal.
Put that into /etc/letsencrypt/renewal-hooks/post/restart:
```shell
#!/bin/bash
systemctl restart maddy
```
And make it executable:
```
$ sudo chmod +x /etc/letsencrypt/renewal-hooks/post/restart
```

## First run

```
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

## postmaster and other user accounts

A mail server is useless without mailboxes, right? Unlike software like postfix
and dovecot, maddy uses "virtual users" by default, meaning it does not care or
know about system users.

Here is the command to create virtual 'postmaster' account, it will prompt you
for a password:
```
$ maddyctl users create postmaster@example.org
```

Note that account names include the domain. When authenticating in the mail
client, full address should be specified as a username as well.

Btw, it is a good idea to learn what else maddyctl can do. Given the
non-standard structure of messages storage, maddyctl is the only way to
comfortably inspect it.

## Optional: Install and use fail2ban

The email world is full of annoying bots just like Web (these annoying scanners
looking for PhpMyAdmin on your blog). fail2ban can help you get rid of them by
temporary blocking offending IPs.

1. Install the fail2ban package using your distribution package manager
For Debian-based distributions:
```
apt-get install fail2ban
```

2. get.sh already installed necessary jail configuration files, but you have to
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
