> Composable all-in-one mail server.

Maddy Mail Server implements all functionality required to run a e-mail
server. It can send messages via SMTP (works as MTA), accept messages via SMTP
(works as MX) and store messages while providing access to them via IMAP.
In addition to that it implements auxiliary protocols that are mandatory
to keep email reasonably secure (DKIM, SPF, DMARC, DANE, MTA-STS).

It replaces Postfix, Dovecot, OpenDKIM, OpenSPF, OpenDMARC and more with one
daemon with uniform configuration and minimal maintenance cost.

**Note:** IMAP storage is "beta". If you are looking for stable and
feature-packed implementation you may want to use Dovecot instead. maddy still
can handle message delivery business.

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)
[![License text](https://img.shields.io/github/license/foxcpp/maddy)](https://github.com/foxcpp/maddy/blob/master/LICENSE)
[![Issues tracker](https://img.shields.io/github/issues/foxcpp/maddy)](https://github.com/foxcpp/maddy)

* [Setup tutorial](https://maddy.email/tutorials/setting-up/)
* [IRC channel](https://webchat.oftc.net/?channels=maddy&uio=MT11bmRlZmluZWQb1)
* [Mailing list](https://lists.sr.ht/~foxcpp/maddy)
