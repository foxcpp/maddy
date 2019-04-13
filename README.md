# maddy

[![builds.sr.ht status](https://builds.sr.ht/~emersion/maddy.svg)](https://builds.sr.ht/~emersion/maddy?)

Fast, cross-platform mail server.

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

Read from `/etc/maddy/maddy.conf` by default.

Start by copying contents of the [maddy.conf][maddy.conf] in this repository.

With this configuration, maddy will create an SQLite3 database for messages in
/var/lib/maddy and use it to store all messages. You need to ensure that this
directory exists and maddy can write to it.

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

* `debug`
  Write verbose logs describing what exactly is happening and how its going.
  Default mode is relatively quiet and still produces useful logs so
  you need that only for debugging purposes.

#### Options usable only at global level

These can be specified only outside of any configuration block.

* `log <targets...>`
  Write log to one of more "targets".
  Target can be one of the following:
  * `stderr`
    Write logs to stderr, this is the default.
  * `syslog`
    Send logs to the local syslog daemon.
  * `off`
    Do nothing. Used to disable logging fully: `log off`
    Can't be combined with other targets.
  * file path
    Write (append) logs to file..

  For example:
  ```
  log off /log
  ```

* `statedir`
  Change directory used for all state-related files.
  Default is $MADDYSTATE environment variable or `/var/lib/maddy` if $MADDYSTATE is not set.
  Default value can be changed using -X linker flag:
  ```
  go build --ldflags '-X github.com/emersion/maddy.defaultStateDirectory=/opt/maddy/state'
  ```

* `libexecdir`
  Change directory where all auxilary binaries are stored.
  Default is $MADDYLIBEXEC environment variable or `/usr/libexec/maddy` if $MADDYLIBEXEC is not set.
  Default value can be changed using -X linker flag:
  ```
  go build --ldflags '-X github.com/emersion/maddy.defaultLibexecDirectory=/opt/maddy/bin'
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
