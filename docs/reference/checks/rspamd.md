# rspamd

The 'rspamd' module implements message filtering by contacting the rspamd
server via HTTP API.

```
check.rspamd {
	tls_client { ... }
	api_path http://127.0.0.1:11333
	settings_id whatever
	tag maddy
	hostname mx.example.org
	io_error_action ignore
	error_resp_action ignore
	add_header_action quarantine
	rewrite_subj_action quarantine
	flags pass_all
}

rspamd http://127.0.0.1:11333
```

## Configuration directives

### tls_client { ... }
Default: not set

Configure TLS client if HTTPS is used. See [TLS configuration / Client](/reference/tls/#client) for details.

---

### api_path _url_
Default: `http://127.0.0.1:11333`

URL of HTTP API endpoint. Supports both HTTP and HTTPS and can include
path element.

---

### settings_id _string_
Default: not set

Settings ID to pass to the server.

---

### tag _string_
Default: `maddy`

Value to send in MTA-Tag header field.

---

### hostname _string_ <br>
Default: value of global directive

Value to send in MTA-Name header field.

---

### io_error_action _action_
Default: `ignore`

Action to take in case of inability to contact the rspamd server.

---

### error_resp_action _action_
Default: `ignore`

Action to take in case of 5xx or 4xx response received from the rspamd server.

---

### add_header_action _action_
Default: `quarantine`

Action to take when rspamd requests to "add header".

X-Spam-Flag and X-Spam-Score are added to the header irregardless of value.

---

### rewrite_subj_action _action_
Default: `quarantine`

Action to take when rspamd requests to "rewrite subject".

X-Spam-Flag and X-Spam-Score are added to the header irregardless of value.

---

### flags _string-list..._
Default: `pass_all`

Flags to pass to the rspamd server.
See [https://rspamd.com/doc/architecture/protocol.html](https://rspamd.com/doc/architecture/protocol.html) for details.
