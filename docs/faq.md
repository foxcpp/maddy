# Frequently Asked Questions

## Why?

For fun. Turned out to be a rather convenient approach to
self-hosted email.

## Is it caddy for email?

No. It was intended to be one but developers quickly acknowledged
the fact email cannot be easily abstracted behind some magic.

## How it compares to MailCow or Mail-In-The-Box?

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

## What is the scope of project?

1. Implement a usable SMTP + Submission server that can both accept
  and send email as secure as possible with todays state of
  relevant protocols.
2. Implement a meaningful subset of IMAP for access to local storage.

## Is there a webmail?

No, at least currently.

I suggest you to check out https://git.sr.ht/~emersion/alps if you
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

## Can I do X with maddy?

Ask on #maddy.

maddy is less feature-packed than other SMTP/IMAP server
implementations but it is not completely useless for anything other than
its default configuration.

## Can you implement X?

"Umbrella" projects like maddy are susceptible to scope
creep unless maintainers apply a lot of skepticism to proposed
features.

If X is essential for providing email security or extends the space of useful
configurations significantly and does not require major design changes -
we can talk, go to #maddy. Otherwise the likely answer is no.

## Are you breaking things between releases?

maddy releases follow Semantic Versioning 2.0.0 specification.
It is expected that 0.X releases may not be compatible with each
other. I attempt to minimize such breakage unless there is a significant
benefit.

## 1.0 when?

When no more backward-incompatible changes will be needed. maddy releases follow
Semantic Versioning 2.0.0 specification.

## maddy is bad name, it is almost impossible to Google!

Call it Maddy Mail Server.
