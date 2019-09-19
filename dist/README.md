# Distribution files for maddy

## systemd unit

`maddy.service` launches using default config path (/etc/maddy/maddy.conf).
`maddy@.service` launches maddy using custom config path. E.g.
`maddy@foo.service` will use /etc/maddy/foo.conf.

Both unit files use DynamicUser to allocate user account for maddy, hence you don't need
to create it explicitly. Also, they use \*Directory options, so required directories
will be created as well.

Additionally, unit files apply strict sandboxing, limiting maddy permissions on
the system to a bare minimum. Subset of these options makes it impossible for
privileged authentication helper binaries to gain required permissions, so you
may have to disable it when using system account-based authentication with
maddy running as a unprivilieged user.

## fail2ban configuration

Configuration files for use with fail2ban. Assume either `backend = systemd` specified
in system-wide configuration or log file written to /var/log/maddy/maddy.log.

See https://github.com/foxcpp/maddy/wiki/fail2ban-configuration for details.
