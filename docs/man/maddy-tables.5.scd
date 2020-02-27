maddy-tables(5) "maddy mail server" "maddy reference documentation"

; TITLE String-string translation

Whenever you need to replace one string with another when handling anything in
maddy, you can use any of the following modules to obtain the replacement
string. They are commonly called "table modules" or just "tables".

# File mapping (file_table)

Declaration:

```
file_table /etc/maddy/aliases
```
or
```
file_table {
	file /etc/maddy/aliases
}
```

This module build string-string mapping from a text file.

File is reloaded every 15 seconds if there are any changes (detected using
modification time).

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
```
