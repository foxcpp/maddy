Distribution files for maddy
------------------------------

**Disclaimer:** Most of the files here are maintained in a "best-effort" way.
That is, they may break or become outdated from time to time. Caveat emptor.

## integration + scripts

These directories provide pre-made configuration snippets suitable for
easy integration with external software.

Usually, this is what you use when you put `import integration/something` in
your config.

## systemd unit

`maddy.service` launches using default config path (/etc/maddy/maddy.conf).
`maddy@.service` launches maddy using custom config path. E.g.
`maddy@foo.service` will use /etc/maddy/foo.conf.

Additionally, unit files apply strict sandboxing, limiting maddy permissions on
the system to a bare minimum. Subset of these options makes it impossible for
privileged authentication helper binaries to gain required permissions, so you
may have to disable it when using system account-based authentication with
maddy running as a unprivileged user.

## fail2ban configuration

Configuration files for use with fail2ban. Assume either `backend = systemd` specified
in system-wide configuration or log file written to /var/log/maddy/maddy.log.

See https://github.com/foxcpp/maddy/wiki/fail2ban-configuration for details.

## logrotate configuration

Meant for logs rotation when logging to file is used.

## vim ftdetect/ftplugin/syntax files

Minimal supplement to make configuration files more readable and help you see
typos in directive names.
