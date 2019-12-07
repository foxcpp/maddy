# get.sh script

get.sh script does the following:

- Makes sure you have a supported version of Go toolchain, if this is not the
  case - it downloads one.
- Downloads and compiles maddy executables.
- Installs maddy executables to /usr/local/bin.
- Installs the dist/ directory contents
- Installs the man/ directory contents
- Install the default configuration.
- Creates maddy user and group.

It is Linux-specific, users of other systems will have to use Manual
installation.

## Environmental variables

Users can be set following environmental variables to control the exact
behavior of the get.sh script.

|  Variable       |  Default value        |  Description |
| --------------- | --------------------- | --- |
| GOVERSION       | 1.13.4                | Go toolchain version to download if the system toolchain is not compatible. |
| MADDYVERSION    | master                | maddy version to download & install. |
| DESTDIR         |                       | Interpret all paths as relative to this directory during installation. |
| PREFIX          | /usr/local            | Installation prefix. |
| SYSTEMDUNITS    | $PREFIX/lib/systemd   | Directory to install systemd units to. |
| CONFDIR         | /etc/maddy            | Path to write configuration files to. |
| FAIL2BANDIR     | /etc/fail2ban         | Path to install fail2ban configs to. |
| SUDO            | sudo                  | Executable to call to elevate privileges during installation. |
