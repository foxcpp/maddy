# Frequently Asked Questions

## I configured maddy as recommended and gmail still puts my messages in spam

Unfortunately, GMail policies are opaque so we cannot tell why this happens.

Verify that you have a rDNS record set for the IP used
by sender server. Also some IPs may just happen to
have bad reputation - check it with various DNSBLs. In this
case you do not have much of a choice but to replace it.

Additionally, you may try marking multiple messages sent from
your domain as "not spam" in GMail UI.

## Message sending fails with `dial tcp X.X.X.X:25: connect: connection timed out` in log

Your provider is blocking outbound SMTP traffic on port 25.

You either have to ask them to unblock it or forward
all outbound messages via a "smart-host".

## What is resource usage of maddy?

For a small personal server, you do not need much more than a
single 1 GiB of RAM and disk space.

## How to setup a catchall address?

https://github.com/foxcpp/maddy/issues/243#issuecomment-655694512

## maddy command prints a "permission denied" error

Run maddy command under the same user as maddy itself.
E.g.
```
sudo -u maddy maddy creds ...
```

## How maddy compares to MailCow or Mail-In-The-Box?

MailCow and MIAB are bundles of well-known email-related software configured to
work together. maddy is a single piece of software implementing subset of what
MailCow and MIAB offer.

maddy offers more uniform configuration system, more lightweight implementation
and has no dependency on Docker or similar technologies for deployment.

maddy may have more bugs than 20 years old battle-tested software.

It is easier to get help with MailCow/MITB since underlying implementations
are well-understood and have active community.

maddy has no Web interface for administration, that is currently done via CLI
utility.

## How maddy IMAP server compares to WildDuck?

Both are "more secure by definition": root access is not required,
implementation is in memory-safe language, etc.

Both support message compression.

Both have first-class Unicode/internationalization support.

WildDuck may offer easier scalability options. maddy does not require you to
setup MongoDB and Redis servers, though. In fact, maddy in its default
configuration has no dependencies besides libc.

maddy has less builtin authentication providers. This means no
app-specific passwords and all that WildDuck lists under point 4 on their
features page.

maddy currently has no admin Web interface, all necessary DB changes are
performed via CLI utility.

## How maddy SMTP server compares to ZoneMTA?

maddy SMTP server has a lot of similarities to ZoneMTA.
Both have powerful mechanisms for message routing (although designed
differently).

maddy does not require MongoDB server for deployment.

maddy has no web interface for queue inspection. However, it can
easily inspected by looking at files in /var/lib/maddy.

ZoneMTA has a number of features that may make it easier to integrate
with HTTP-based services. maddy speaks standard email protocols (SMTP,
Submission).

## Is there a webmail?

No, at least currently.

I suggest you to check out [alps](https://git.sr.ht/~migadu/alps) if you
are fine with alpha-quality but extremely easy to deploy webmail.

## Is there a content filter (spam filter)?

No. maddy moves email messages around, it does not classify
them as bad or good with the notable exception of sender policies.

It is possible to integrate rspamd using 'rspamd' module. Just add
`rspamd` line to `checks` in `local_routing`, it should just work
in most cases.

## Is it production-ready?

maddy is considered "beta" quality. Several people use it for personal email.

## Single process makes it unreliable. This is dumb!

This is a compromise between ease of management and reliability. Several
measures are implemented in code base in attempt to reduce possible effect
of bugs in one component.

Besides, you are not required to use a single process, it is easy to launch
maddy with a non-default configuration path and connect multiple instances
together using off-the-shelf protocols.
