# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Simple, fast IMAP+SMTP mail server.

Maddy implements Mail Transfer agent (MTA), Mail Submission Agent (MSA) and
IMAP server functionality in one application.

**⚠️ Warning:** maddy is in development, many important features are missing, there
are bugs and performance can be bad.

## Getting started

### Installation

You need Go 1.11 or newer. A C compiler is required for SQLite3 storage support
(can be disabled using `nosqlite3` tag). 

```shell
go get github.com/emersion/maddy/cmd/maddy
```

You can also compile and install helper binaries from
[cmd/maddy-pam-helper](cmd/maddy-pam-helper/README.md) and
[cmd/maddy-shadow-helper](cmd/maddy-shadow-helper/README.md). See corresponding
README files for details.

### Configuration

Start by copying contents of the [maddy.conf](maddy.conf) to
`/etc/maddy/maddy.conf` (default configurtion location).

With this configuration, maddy will create an SQLite3 database for messages in
/var/lib/maddy and use it to store all messages. You need to ensure that this
directory exists and maddy can write to it.

Configuration syntax, high-level structure, and all implemented options are
documented in maddy.conf(5) man page.

You can view page source [here](maddy.conf.5.scd) (it is readable!) or
generate man page using [scdoc](https://git.sr.ht/~sircmpwn/scdoc) utliity:
```
scdoc < maddy.conf.5.scd > maddy.conf.5
```

## License

MIT
