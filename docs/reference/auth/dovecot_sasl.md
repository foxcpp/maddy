# Dovecot SASL

The 'auth.dovecot_sasl' module implements the client side of the Dovecot
authentication protocol, allowing maddy to use it as a credentials source.

Currently SASL mechanisms support is limited to mechanisms supported by maddy
so you cannot get e.g. SCRAM-MD5 this way.

```
auth.dovecot_sasl {
	endpoint unix://socket_path
}

dovecot_sasl unix://socket_path
```

## Configuration directives

### endpoint _schema://address_
Default: not set

Set the address to use to contact Dovecot SASL server in the standard endpoint
format.

`tcp://10.0.0.1:2222` for TCP, `unix:///var/lib/dovecot/auth.sock` for Unix
domain sockets.
