# Modules introduction

maddy is built of many small components called "modules". Each module does one
certain well-defined task. Modules can be connected to each other in arbitrary
ways to achieve wanted functionality. Default configuration file defines
set of modules that together implement typical email server stack.

To specify the module that should be used by another module for something, look
for configuration directives with "module reference" argument. Then
put the module name as an argument for it. Optionally, if referenced module
needs that, put additional arguments after the name. You can also put a
configuration block with additional directives specifing the module
configuration.

Here are some examples:

```
smtp ... {
    # Deliver messages to the 'dummy' module with the default configuration.
    deliver_to dummy

    # Deliver messages to the 'target.smtp' module with
    # 'tcp://127.0.0.1:1125' argument as a configuration.
    deliver_to smtp tcp://127.0.0.1:1125

    # Deliver messages to the 'queue' module with the specified configuration.
    deliver_to queue {
        target ...
        max_tries 10
    }
}
```

Additionally, module configuration can be placed in a separate named block
at the top-level and referenced by its name where it is needed.

Here is the example:
```
storage.imapsql local_mailboxes {
    driver sqlite3
    dsn all.db
}

smtp ... {
    deliver_to &local_mailboxes
}
```

It is recommended to use this syntax for modules that are 'expensive' to
initialize such as storage backends and authentication providers.

For top-level configuration block definition, syntax is as follows:
```
namespace.module_name config_block_name... {
    module_configuration
}
```
If config\_block\_name is omitted, it will be the same as module\_name. Multiple
names can be specified. All names must be unique.

Note the "storage." prefix. This is the actual module name and includes
"namespace". It is a little cheating to make more concise names and can
be omitted when you reference the module where it is used since it can
be implied (e.g. putting module reference in "check{}" likely means you want
something with "check." prefix)

Usual module arguments can't be specified when using this syntax, however,
modules usually provide explicit directives that allow to specify the needed
values. For example 'sql sqlite3 all.db' is equivalent to
```
storage.imapsql {
    driver sqlite3
    dsn all.db
}
```

