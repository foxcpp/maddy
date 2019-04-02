# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Fast, cross-platform mail server.

Inspired from [Caddy](https://github.com/mholt/caddy).

## Installation

```shell
go get github.com/emersion/maddy/cmd/maddy
```

Build tags:
* `nosqlite3`
  Disable SQLite3 support in go-imap-sql (enabled by default). Saves around 9
  MiB of binary size.
* `mysql`
  Include support for MySQL driver in go-imap-sql.
* `postgresql`
  Include support for PostgreSQL driver in go-imap-sql.

## Configuration

### Syntax

Maddy uses configuration format similar (but not the same!) to Caddy's
Caddyfile.  You may want to read [Caddyfile page from Caddy docs](https://caddyserver.com/docs/caddyfile).

Notable differences from Caddy's format:
* Directives can't span multiple lines
* Arbitrary nesting is supported
* You can't omit braces if you have only one configuration block

### Modularity

Maddy does have a module-based design and you specify in configuration modules
you want to use and how you want them to work.

Generic syntax for configuration block is as follows:
```
module_name instance_name {
    configuration_directives
}
```
You can omit braces if there is no configuration directives in block:
```
module_name instance_name
```

`instance_name` is the unique name of the configuration block. It is used when
you need to refer to the module from different place in configuration (e.g.
configure SMTP to deliver mail to certain specific storage)

You can omit `instance_name`. If there is only one module config. block - it
will get name the same as `module_name`. However, you can't define multiple
config. blocks this way because names should be unique.

### Global options

Certain options can be specified outside of any configuration block. They
specify defaults for all configuration blocks unless they override them. Below
are options that can be used like that:

* `hostname <domain>`
  Specify local hostname for all modules. Relevant for SMTP endpoints and queues.

* `tls <cert_file> <pkey_file>`
  Default TLS certificate to use. See
  [CONFIG_REFERENCE.md](CONFIG_REFERENCE.md) for details.

### Defaults

Maddy provides reasonable defaults so you can start using it without spending
hours writing configuration files. All you need it so define smtp and imap
modules in your configuration, configure TLS (see below) and set domain name.

Here is the minimal example to get you started:
```
tls cert_file pkey_file
hostname emersion.fr

imap imap://0.0.0.0 imaps://0.0.0.0
smtp smtp://0.0.0.0:25
submission smtp://0.0.0.0:587 smtps://0.0.0.0:465
```
Don't forget to use actual values instead of placeholders.

With this configuration, maddy will create an SQLite3 database for messages in
current directory and use it to store all messages.

### go-imap-sql: Database location

If you don't like SQLite3 or don't want to have it in the current directory,
you can override the configuration of the default module.

See [go-imap-sql repository](https://github.com/foxcpp/go-imap-sql) for
information on RDBMS support.

```
sql default {
    driver sqlite3
    dsn file_path
}
```

You can then replace SQL driver and DSN values. Note that maddy needs to be
built with a build tag corresponding to the name of the used driver (`mysql`,
`postgresql`) for SQL engines other than sqlite3.

DSN is a driver-specific value that describes the database to use.
For SQLite3 this is just a file path.
For MySQL: https://github.com/go-sql-driver/mysql#dsn-data-source-name
For PostgreSQL: https://godoc.org/github.com/lib/pq#hdr-Connection_String_Parameters

Note that you can also change default DSN or SQL driver during compilation
by building maddy using following command:
```shell
go build -ldflags "-X github,com/emersion/maddy.defaultDriver=DRIVER -X github.com/emersion/maddy.defaultDsn=DSN"
```

### TLS

Currently, maddy doesn't implement any form of automatic TLS like Caddy. But
since we don't want to have insecure defaults we require users to either
manually configure TLS or disable it explicitly.

Valid variants of TLS config directive:

Disable TLS:
```
tls off  # this is insecure!
```

Use temporary self-signed certificate (useful for testing):
```
tls self_signed   # this is insecure too!
```

Use specified certificate and private key:
```
tls cert_file pkey_file
```

### SMTP pipeline & other customization options 

List of all configuration options and all modules you can use is in
[CONFIG_REFERENCE.md](CONFIG_REFERENCE.md) file.

## License

MIT
