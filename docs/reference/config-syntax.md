# Configuration files syntax

**Note:** This file is a technical document describing how
maddy parses configuration files.

Configuration consists of newline-delimited "directives". Each directive can
have zero or more arguments.

```
directive0
directive1 arg0 arg1
```

Any line starting with # is ignored. Empty lines are ignored too.

## Quoting

Strings with whitespace should be wrapped into double quotes to make sure they
will be interpreted as a single argument.

```
directive0 two arguments
directive1 "one argument"
```

String wrapped in quotes may contain newlines and they will not be interpreted
as a directive separator.

```
directive0 "one long big
argument for directive0"
```

Quotes and only quotes can be escaped inside literals: \\"

Backslash can be used at the end of line to continue the directve on the next
line.

## Blocks

A directive may have several subdirectives. They are written in a {-enclosed
block like this:
```
directive0 arg0 arg1 {
    subdirective0 arg0 arg1
    subdirective1 etc
}
```

Subdirectives can have blocks too.

```
directive0 {
    subdirective0 {
        subdirective2 {
            a
            b
            c
        }
    }
    subdirective1 { }
}
```

Level of nesting is limited, but you should never hit the limit with correct
configuration.

In most cases, an empty block is equivalent to no block:
```
directive { }
directive2 # same as above
```

## Environment variables

Environment variables can be referenced in the configuration using either
{env:VARIABLENAME} syntax.

Non-existent variables are expanded to empty strings and not removed from
the arguments list.  In the following example, directive0 will have one argument
independently of whether VAR is defined.

```
directive0 {env:VAR}
```

Parse is forgiving and incomplete variable placeholder (e.g. '{env:VAR') will
be left as-is. Variables are expanded inside quotes too.

## Configuration macros (variables)

Macros let you reuse one or more configuration arguments. Declare a macro at
the top level, before the directive that references it. If a snippet contains
a macro reference, declare the macro before the snippet. Put `=` after the macro
name and list its value or values after that:

```
$(mail_domains) = example.org example.net
$(mailbox) = "Example User"

some_directive $(mail_domains)
mailbox_directive $(mailbox)
another_directive mail/$(mailbox)/Inbox
```

When a macro reference is a complete argument, it expands to all of the
arguments in its definition. In this example, `some_directive` receives two
arguments: `example.org` and `example.net`. Quoting follows the normal
configuration rules, so `mailbox_directive` receives one argument,
`Example User`.

A macro can also be embedded in a larger argument, as in
`mail/$(mailbox)/Inbox`. In this form, the macro must have exactly one value;
using a macro with multiple values inside another argument is an error. An
undefined macro used as a complete argument contributes no arguments, while an
undefined macro embedded in an argument is replaced with an empty string.

Macro declarations are only allowed at the top level, and macro references
cannot be used as directive names. Macros use `$(name)` syntax and are separate
from the `{env:NAME}` environment-variable expansion described above.

## Snippets & imports

You can reuse blocks of configuration by defining them as "snippets". Snippet
is just a directive with a block, declared tp top level (not inside any blocks)
and with a directive name wrapped in curly braces.

```
(snippetname) {
    a
    b
    c
}
```

The snippet can then be referenced using 'import' meta-directive.

```
unrelated0
unrelated1
import snippetname
```

The above example will be expanded into the following configuration:

```
unrelated0
unrelated1
a
b
c
```

Import statement also can be used to include content from other files. It works
exactly the same way as with snippets but the file path should be used instead.
The path can be either relative to the location of the currently processed
configuration file or absolute. If there are both snippet and file with the
same name - snippet will be used.

```
# /etc/maddy/tls.conf
tls long_path_to_certificate long_path_to_private_key

# /etc/maddy/maddy.conf
smtp tcp://0.0.0.0:25 {
    import tls.conf
}
```

```
# Expanded into:
smtp tcp://0.0.0.0:25 {
    tls long_path_to_certificate long_path_to_private_key
}
```

The imported file can introduce new snippets and they can be referenced in any
processed configuration file.

## Duration values

Directives that accept duration use the following format: A sequence of decimal
digits with an optional fraction and unit suffix (zero can be specified without
a suffix). If multiple values are specified, they will be added.

Valid unit suffixes: "h" (hours), "m" (minutes), "s" (seconds), "ms" (milliseconds).
Implementation also accepts us and ns for microseconds and nanoseconds, but these
values are useless in practice.

Examples:
```
1h
1h 5m
1h5m
0
```

## Data size values

Similar to duration values, but fractions are not allowed and suffixes are different.

Valid unit suffixes: "G" (gibibyte, 1024^3 bytes), "M" (mebibyte, 1024^2 bytes),
"K" (kibibyte, 1024 bytes), "B" or "b" (byte).

Examples:
```
32M
3M 5K
5b
```

Also note that the following is not valid, unlike Duration values syntax:
```
32M5K
```

## Address Definitions

Maddy configuration uses URL-like syntax to specify network addresses.

- `unix://file_path` – Unix domain socket. Relative paths are relative to runtime directory (`/run/maddy`).
- `tcp://ADDRESS:PORT` – TCP/IP socket.
- `tls://ADDRESS:PORT` – TCP/IP socket using TLS.

## Dummy Module

No-op module. It doesn't need to be configured explicitly and can be referenced
using "dummy" name. It can act as a delivery target or auth.
provider. In the latter case, it will accept any credentials, allowing any
client to authenticate using any username and password (use with care!).


