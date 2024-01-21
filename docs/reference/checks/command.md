# System command filter

This module executes an arbitrary system command during a specified stage of
checks execution.

```
command executable_name arg0 arg1 ... {
	run_on body

	code 1 reject
	code 2 quarantine
}
```

## Arguments

The module arguments specify the command to run. If the first argument is not
an absolute path, it is looked up in the Libexec Directory (/usr/lib/maddy on
Linux) and in $PATH (in that ordering). Note that no additional handling
of arguments is done, especially, the command is executed directly, not via the
system shell.

There is a set of special strings that are replaced with the corresponding
message-specific values:

- `{source_ip}` – IPv4/IPv6 address of the sending MTA.
- `{source_host}` – Hostname of the sending MTA, from the HELO/EHLO command.
- `{source_rdns}` – PTR record of the sending MTA IP address.
- `{msg_id}` – Internal message identifier. Unique for each delivery.
- `{auth_user}` – Client username, if authenticated using SASL PLAIN
- `{sender}` – Message sender address, as specified in the MAIL FROM SMTP command.
- `{rcpts}` – List of accepted recipient addresses, including the currently handled
  one.
- `{address}` – Currently handled address. This is a recipient address if the command
  is called during RCPT TO command handling (`run_on rcpt`) or a sender
  address if the command is called during MAIL FROM command handling (`run_on
  sender`).

If value is undefined (e.g. `{source_ip}` for a message accepted over a Unix
socket) or unavailable (the command is executed too early), the placeholder
is replaced with an empty string. Note that it can not remove the argument.
E.g. `-i {source_ip}` will not become just `-i`, it will be `-i ""`

Undefined placeholders are not replaced.

## Command stdout

The command stdout must be either empty or contain a valid RFC 5322 header.
If it contains a byte stream that does not look a valid header, the message
will be rejected with a temporary error.

The header from stdout will be **prepended** to the message header.

## Configuration directives

### run_on `conn` | `sender` | `rcpt` | `body`
Default: `body`

When to run the command. This directive also affects the information visible
for the message.

- `conn`<br>
    Run before the sender address (MAIL FROM) is handled.<br>
    **Stdin**: Empty <br>
    **Available placeholders**: {source_ip}, {source_host}, {msg_id}, {auth_user}.

- `sender`<br>
    Run during sender address (MAIL FROM) handling.<br>
    **Stdin**: Empty <br>
    **Available placeholders**: conn placeholders + {sender}, {address}.
    The {address} placeholder contains the MAIL FROM address.

- `rcpt`<br>
    Run during recipient address (RCPT TO) handling. The command is executed
    once for each RCPT TO command, even if the same recipient is specified
    multiple times.<br>
    **Stdin**: Empty <br>
    **Available placeholders**: sender placeholders + {rcpts}.
    The {address} placeholder contains the recipient address.

- `body`<br>
    Run during message body handling.<br>
    **Stdin**: The message header + body <br>
    **Available placeholders**: all except for {address}.

---

### code _integer_ ignore <br>code _integer_ quarantine <br>code _integer_ reject _smtp-code_ _smtp-enhanced-code_ _smtp-message_

This directive specifies the mapping from the command exit code _integer_ to
the message pipeline action.

Two codes are defined implicitly, exit code 1 causes the message to be rejected
with a permanent error, exit code 2 causes the message to be quarantined. Both
actions can be overridden using the 'code' directive.

