# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Simple, fast, secure all-in-one mail server.

**⚠️ Disclaimer: maddy is in development, many planned features are
missing, bugs are waiting to eat your messages and incompatible 
changes happen from time to time**

Join [##maddy on irc.freenode.net](https://webchat.freenode.net/##maddy), if you
have any questions or just want to talk about maddy.

## Table of contents

- [Features](#features)
- [Installation](#installation)
- [Building from source](#building-from-source)
  - [Dependencies](#dependencies)
  - [Building](#building)
- [Quick start](#quick-start)
  - [systemd unit](#systemd-unit)
- [Documentation](#documentation)
- [Contributing](#contributing)
- [License](#license)

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
here:
https://github.com/foxcpp/maddy/wiki/Tutorial:-Setting-up-a-mail-server-with-maddy

### Manual installation

#### Dependencies 

- [Go](https://golang.org) toolchain (1.13 or newer)
  
  If your distribution ships an outdated Go version, you can use
  following commands to get a newer version:
  ```
  go get golang.org/dl/go1.13
  go1.13 download
  ```
  Then use `go1.13` instead of `go` in commands below.
  
- C compiler (**optional**, set CGO_ENABLED env. variable to 0 to disable)

  Required for SQLite3-based storage (default configuration) and PAM
  authentication.

#### Building

First, make sure Go Modules support is enabled:
```
export GO111MODULE=on
```

There are two binaries to install, server itself and DB management 
utility. Use the following command to install them:
```
go get github.com/foxcpp/maddy/cmd/{maddy,maddyctl}@master
```

Executables will be placed in the $GOPATH/bin directory (defaults to
$HOME/go/bin).

#### Quick start

*Note*: explaination below is short and assumes that you already have
basic ideas about how email works.

1. Install maddy and maddyctl (see above)
2. Copy maddy.conf from this repo to /etc/maddy/maddy.conf
3. Create /run/maddy and /var/lib/maddy, make sure they are writable
   for the maddy user. Though, you don't have to use system directories,
   see `maddy -help`.
4. Open maddy.conf with ~~vim~~your favorite editor and change 
   the following:
- `tls ...` 
  Change to paths to TLS certificate and key.
- `$(hostname)`
  Server identifier. Put your domain here if you have only one server.
- `$(primary_domain)`
  Put the "main" domain you are handling messages for here.
5. Run the executable.
6. On first start-up server will generate a RSA-2048 keypair for DKIM and tell
   you where file with DNS record text is placed. You need to add it to your
   zone to make signing work.
7. Create user accounts you need using `maddyctl`:
   ```
   maddyctl users create foxcpp
   ```

Congratulations, now you have your working mail server.
IMAP endpoint is on port 993 with TLS enforced ("implicit TLS").
SMTP endpoint is on port 465 with TLS enforced ("implicit TLS").

### systemd unit

You can use the systemd unit file from the [dist/](dist) directory in
this repo. It will automatically set-up user account and directories.
Additionally, it will apply strict sandbox to maddy to ensure additional
security.

You need a relatively new systemd version (235+) both of these things to work
properly. Otherwise, you still have to manually create a user account and the
state + runtime directories with read-write permissions for the maddy user. 

## Documentation

Reference documentation is maintained as a set of man pages
in the [scdoc](https://git.sr.ht/~sircmpwn/scdoc) format.
Rendered pages can be browsed [here](https://foxcpp.dev/maddy-reference).

Tutorials and misc articles will be added to
the [project wiki](https://github.com/foxcpp/maddy/wiki).

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


