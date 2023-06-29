# Envelope sender / recipient rewriting

`replace_sender` and `replace_rcpt` modules replace SMTP envelope addresses
based on the mapping defined by the table module (maddy-tables(5)). It is possible
to specify 1:N mappings. This allows, for example, implementing mailing lists.

The address is normalized before lookup (Punycode in domain-part is decoded,
Unicode is normalized to NFC, the whole string is case-folded).

First, the whole address is looked up. If there is no replacement, local-part
of the address is looked up separately and is replaced in the address while
keeping the domain part intact. Replacements are not applied recursively, that
is, lookup is not repeated for the replacement.

Recipients are not deduplicated after expansion, so message may be delivered
multiple times to a single recipient. However, used delivery target can apply
such deduplication (imapsql storage does it).

Definition:

```
replace_rcpt <table> [table arguments] {
	[extended table config]
}
replace_sender <table> [table arguments] {
	[extended table config]
}
```

Use examples:

```
modify {
	replace_rcpt file /etc/maddy/aliases
	replace_rcpt static {
		entry a@example.org b@example.org
		entry c@example.org c1@example.org c2@example.org
	}
	replace_rcpt regexp "(.+)@example.net" "$1@example.org"
	replace_rcpt regexp "(.+)@example.net" "$1@example.org" "$1@example.com"
}
```

Possible contents of /etc/maddy/aliases in the example above:

```
# Replace 'cat' with any domain to 'dog'.
# E.g. cat@example.net -> dog@example.net
cat: dog

# Replace cat@example.org with cat@example.com.
# Takes priority over the previous line.
cat@example.org: cat@example.com

# Using aliases in multiple lines
cat2: dog
cat2: mouse
cat2@example.org: cat@example.com
cat2@example.org: cat@example.net
# Comma-separated aliases in multiple lines
cat3: dog , mouse
cat3@example.org: cat@example.com , cat@example.net
```