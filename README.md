# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Simple, fast, secure all-in-one mail server.

**⚠️ Disclaimer: maddy is in development, many planned features are
missing, bugs are waiting to eat your messages and incompatible 
changes happen from time to time**

## Features

* Comprehensive & secure
  - IMAP4rev1 & SMTP server in one binary
  - [DKIM][dkim] signing and verification
  - [SPF][spf] policy enforcement
  - [DMARC][dmarc] policy enforcement (experimental,
    enable with `dmarc on` in smtp config)
  - [MTA-STS][mtasts] policy enforcement
* Simple to configure
  - Two steps (excluding messing with DNS) to get your own
    _secure_ mail server running
  - Virtual users > system users
* Fast
  - Optimized for concurrency
  - Single process model allows more efficient implementation
* Useful
  - [Subaddressing][subaddr] support 
  - Messages compression (LZ4, Zstd)
 
Planned:
- [Backscatter][backscatter] filtering (BATV) (#106)
- Address aliases (#82, #18)
- Zero-configuration full-text search (foxcpp/go-imap-sql#21)
- Milter protocol support (#16)
- DANE (#50)
- Server-side messages encryption (#75)
- [JMAP](https://jmap.io) (#19)

## Installation & configuration

Detailed explaination of what you need to do to get it running can be found
[here][setup-tutorial]. 

## Documentation

Reference documentation is maintained as a set of man pages
in the [scdoc](https://git.sr.ht/~sircmpwn/scdoc) format.
Rendered pages can be browsed [here](https://foxcpp.dev/maddy-reference).

Tutorials and misc articles will be added to
the [project wiki](https://github.com/foxcpp/maddy/wiki).

## Community

There is IRC channel on freenode.net named
[##maddy](https://webchat.freenode.net/##maddy). You can join it if you have
any questions or just want to talk.

Also there is public mailing list for maddy-related discussions on
https://lists.sr.ht/~foxcpp/maddy. You can use it too.

## Contributing

See [.github/CONTRIBUTING.md](.github/CONTRIBUTING.md).

## License

The code is under MIT license. See [LICENSE](LICENSE) for more information.

[dkim]: https://blog.returnpath.com/how-to-explain-dkim-in-plain-english-2/
[spf]: https://blog.returnpath.com/how-to-explain-spf-in-plain-english/
[dmarc]: https://blog.returnpath.com/how-to-explain-dmarc-in-plain-english/
[mtasts]: https://www.hardenize.com/blog/mta-sts
[subaddr]: https://en.wikipedia.org/wiki/Email_address#Sub-addressing
[backscatter]: https://en.wikipedia.org/wiki/Backscatter_(e-mail)

[setup-tutorial]: https://github.com/foxcpp/maddy/wiki/Tutorial:-Setting-up-a-mail-server-with-maddy
