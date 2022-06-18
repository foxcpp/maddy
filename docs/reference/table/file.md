# File 

table.file module builds string-string mapping from a text file.

File is reloaded every 15 seconds if there are any changes (detected using
modification time). No changes are applied if file contains syntax errors.

Definition:
```
file <file path>
```
or
```
file {
	file <file path>
}
```

Usage example:
```
# Resolve SMTP address aliases using text file mapping.
modify {
	replace_rcpt file /etc/maddy/aliases
}
```

## Syntax

Better demonstrated by examples:

```
# Lines starting with # are ignored.

# And so are lines only with whitespace.

# Whenever 'aaa' is looked up, return 'bbb'
aaa: bbb

	# Trailing and leading whitespace is ignored.
	ccc: ddd

# If there is no colon, the string is translated into ""
# That is, the following line is equivalent to
#	aaa:
aaa

# If the same key is used multiple times - table.file will return
# multiple values when queries.
ddd: firstvalue
ddd: secondvalue

# Alternatively, multiple values can be specified
# using a comma. There is no support for escaping
# so you would have to use a different format if you require
# comma-separated values.
ddd: firstvalue, secondvalue
```

