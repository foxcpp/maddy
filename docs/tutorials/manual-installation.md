# Manual installation & configuration

## Dependencies

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

## Building

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

## Configuration

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
   maddyctl users create foxcpp@example.org
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
