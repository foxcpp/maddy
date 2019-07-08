# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Simple, fast IMAP+SMTP mail server.

Maddy implements Mail Transfer agent (MTA), Mail Submission Agent (MSA), Mail Delivery Agent (MDA) and
IMAP server functionality in one application.

**⚠️ Warning:** maddy is in development, many important features are missing, there
are bugs and performance can be bad.

Feel free to join the IRC channel: ##emersion on irc.freenode.net.

## Getting started

### Installation

You need Go 1.11.4 or newer. A C compiler is required for SQLite3 storage support, 
you can disable SQLite3 support by passing `-tags 'nosqlite3'` to `go build`. 
Also you need to enable modules support to get the right version. Set
`GO111MODULE` environment variable to `on`.

```shell
export GO111MODULE=on
go get github.com/emersion/maddy/cmd/maddy@master
```

You can also compile and install helper binaries from
[cmd/maddy-pam-helper](cmd/maddy-pam-helper/README.md) and
[cmd/maddy-shadow-helper](cmd/maddy-shadow-helper/README.md). See corresponding
README files for details.

### Configuration

Start by copying contents of the [maddy.conf](maddy.conf) to
`/etc/maddy/maddy.conf` (default configuration location).

With this configuration, maddy will create an SQLite3 database for messages in
/var/lib/maddy and use it to store all messages. You need to ensure that this
directory exists and maddy can write to it.

Configuration syntax, high-level structure, and all implemented options are
documented in maddy.conf(5) man page.

You can view page source [here](maddy.conf.5.scd) (it is readable!) or
generate man page using [scdoc](https://git.sr.ht/~sircmpwn/scdoc) utility:
```
scdoc < maddy.conf.5.scd > maddy.conf.5
```

## go-imap-sql database management

go-imap-sql is the main storage backend used by maddy. You might want to take a
look at https://github.com/foxcpp/go-imap-sql and
https://github.com/foxcpp/go-imap-sql/tree/master/cmd/imapsql-ctl for how to
configure and use it. 
```
export GO111MODULE=on
go get github.com/foxcpp/go-imap-sql/cmd/imapsql-ctl
```

You need imapsql-ctl tool to create user accounts. Here is the command to use:
```
imapsql-ctl --driver DRIVER --dsn DSN users create NAME
```

## License

MIT
