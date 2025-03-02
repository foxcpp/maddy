# Header Modifiers

## Adding a new header

`add_header` module modifies the message by adding an header.

Note: the header must be present in the message prior to the execution of the modifier. 
If the header already exist it will result in an error.


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
