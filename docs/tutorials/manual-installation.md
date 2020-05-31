# Manual installation & configuration

## Dependencies

- [Go](https://golang.org) toolchain (1.14 or newer)

  If your distribution ships an outdated Go version, you can use
  following commands to get a newer version:
  ```
  go get golang.org/dl/go1.14
  go1.14 download
  ```
  Then use `go1.14` instead of `go` in commands below.

- C compiler (**optional**, set CGO_ENABLED env. variable to 0 to disable)

  Required for SQLite3-based storage (default configuration) and PAM
  authentication.

## Building

Clone maddy repo:
```
git clone https://github.com/foxcpp/maddy.git
cd maddy
```

There are two binaries to build, server itself and DB management
utility. Use the following commands to install them:
```
go build ./cmd/maddyctl
go build ./cmd/maddy
```

Executables will be placed in the current directory. Copy them to
/usr/local/bin or whatever directory you them to be in.

## Configuration

*Note*: explaination below is short and assumes that you already have
basic ideas about how email works.

1. Install maddy and maddyctl (see above)
2. Copy maddy.conf from the repo to /etc/maddy/maddy.conf
3. Create /run/maddy and /var/lib/maddy, make sure they are writable
   for the maddy user. Though, you don't have to use system directories,
   see `maddy -help`.
4. Open maddy.conf with your favorite editor and change
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
   maddyctl creds create foxcpp@example.org
   maddyctl imap-acct create foxcpp@example.org
   ```

Congratulations, now you have your working mail server.
IMAP endpoint is on port 993 with TLS enforced ("implicit TLS").
SMTP endpoint is on port 465 with TLS enforced ("implicit TLS").

### systemd unit

You can use the systemd unit file from the [dist/](dist) directory in
the repo to supervise the server process and start it at boot.
