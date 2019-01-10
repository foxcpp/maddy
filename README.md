# maddy

Fast, cross-platform mail server.

Inspired from [Caddy](https://github.com/mholt/caddy).

## Features

* Easy configuration with Maddyfile (same as Caddyfile)
* Automatic TLS and DKIM support
* Transparent PGP support
* Runs anywhere with no external dependencies (not even libc)

## Usage

```shell
go get github.com/emersion/maddy/...
```

Create a file named `Maddyfile`:

```
imaps://127.0.0.1:1993 {
	proxy imaps://mail.example.org
	tls self_signed
	compress
	pgp
}

smtps://127.0.0.1:1025 {
	proxy smtps://mail.example.org
	tls self_signed
	pgp
}
```

## License

MIT
