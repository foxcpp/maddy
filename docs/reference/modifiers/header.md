# Header Modifiers

## Adding a new header

`add_header` module modifies the message by adding an header.

Note: Adding a header with an existing key will create multiple entries for that key in the message.

Definition:

```
add_header <headerName> <headerValue>
```

Use examples:

```
modify {
	add_header X-My-Header "header value"
}
```
