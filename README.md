maddy [![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?) [![codecov](https://img.shields.io/codecov/c/github/foxcpp/maddy)](https://codecov.io/gh/foxcpp/maddy)
=====================

Composable all-in-one mail server.

## Features

- IMAP4rev1 & SMTP server in one binary
- Comprehensive & Secure
  - [DKIM][dkim] signing and verification
  - [SPF][spf] policy enforcement
  - [DMARC][dmarc] policy enforcement
  - [MTA-STS][mtasts] policy enforcement
  - [DANE][dane] support
  - Built-in [STARTTLS Everywhere][sts-preload] rules support
- Simple to deploy
  - Two steps (excluding messing with DNS) to get your own
    _secure_ mail server running
  - Virtual users > system users
  - Single daemon that is easy to manage
- Fast
  - Optimized for concurrency
  - Single process model allows more efficient implementation
- Useful
  - [Subaddressing][subaddr] support
  - [DNSBL][dnsbl] checking support
  - Messages compression (LZ4, Zstd)
  - First-class Unicode support (SMTPUTF8, IDNA2008)
- Flexible
  - Intuitive syntax for complex message routing (SMTP)
  - Same meaningful configuration scheme for all filters
  - Any builtin functionality can be replaced with
    third-party implementation if it you need it

## Installation & configuration

Detailed explaination of what you need to do to get it running can be found
[here][setup-tutorial].

## Documentation

The full documentation is published [here](https://foxcpp.dev/maddy/)

Manual pages with reference documentation will be installed by build.sh if
scdoc utility is available on the system.

## Community

There is IRC channel on freenode.net named
[#maddy](https://webchat.freenode.net/#maddy). You can join it if you have
any questions or just want to talk.

Also there is public mailing list for maddy-related discussions on
https://lists.sr.ht/~foxcpp/maddy. You can use it too.

## License

The code is under MIT license. See [LICENSE](LICENSE) for more information.


[dkim]: https://www.validity.com/blog/how-to-explain-dkim-in-plain-english/
[spf]: https://blog.returnpath.com/how-to-explain-spf-in-plain-english/
[dmarc]: https://blog.returnpath.com/how-to-explain-dmarc-in-plain-english/
[mtasts]: https://www.hardenize.com/blog/mta-sts
[dane]: https://halon.io/blog/what-is-dane/
[sts-preload]: https://starttls-everywhere.org/
[subaddr]: https://en.wikipedia.org/wiki/Email_address#Sub-addressing
[dnsbl]: https://en.wikipedia.org/wiki/DNSBL
[backscatter]: https://en.wikipedia.org/wiki/Backscatter_(e-mail)

[setup-tutorial]: https://foxcpp.dev/maddy/tutorials/setting-up/
