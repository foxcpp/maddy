# Regexp rewrite table

The 'regexp' module implements table lookups by applying a regular expression
to the key value. If it matches - 'replacement' value is returned with $N
placeholders being replaced with corresponding capture groups from the match.
Otherwise, no value is returned.

The regular expression syntax is the subset of PCRE. See
[https://golang.org/pkg/regexp/syntax](https://golang.org/pkg/regexp/syntax)/ for details.

```
table.regexp <regexp> [replacement] {
	full_match yes
	case_insensitive yes
	expand_placeholders yes
}
```

Note that [replacement] is optional. If it is not included - table.regexp
will return the original string, therefore acting as a regexp match check.
This can be useful in combination in destination\_in for
advanced matching:
```
destination_in regexp ".*-bounce+.*@example.com" {
	...
}
```

## Configuration directives

***Syntax***: full\_match _boolean_ <br>
***Default***: yes

Whether to implicitly add start/end anchors to the regular expression.
That is, if 'full\_match' is yes, then the provided regular expression should
match the whole string. With no - partial match is enough.

***Syntax***: case\_insensitive _boolean_ <br>
***Default***: yes

Whether to make matching case-insensitive.

***Syntax***: expand\_placeholders _boolean_ <br>
***Default***: yes

Replace '$name' and '${name}' in the replacement string with contents of
corresponding capture groups from the match.

To insert a literal $ in the output, use $$ in the template.

# Identity table (table.identity)

The module 'identity' is a table module that just returns the key looked up.

```
table.identity { }
```

