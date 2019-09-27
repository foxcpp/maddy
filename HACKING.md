## Design goals

- **Make it easy to deploy**
  Minimal configuration changes should be required to get typical mail server
  running. Though, it is important to avoid making guesses for "zero
  configuration". Wrong guess is worse than no guess.

- **Provide 80% of needed components**
  E-mail has evolved into a huge mess. With a single package to do one thing, it
  quickly turns into maintenance nightmare. Put all stuff mail server typically
  needs into a single package. Though, leave controversial or highly opinionated
  stuff out, don't force people to do things our way (see next point).

- **Interoperate with existing software**
  Implement (de-facto) standard protocols not only for clients but also for
  various server-side helper software (content filters, etc).

- **Be secure but interoperable**
  Verify DKIM signatures by default, use DMRAC policies by default, etc. This
  makes default setup as secure as possible while maintaining reasonable
  interoperability. Users can configure maddy to be more strict at cost of
  interoperability.

- **Acheive flexibility through composability**
  Allow to connect components in arbitrary ways instead of restricting users to
  predefined templates.

- **Use Go concurrency features to full extent**
  Do as much I/O as possible in parallel to minimize latencies. It is silly to
  not do so when it is possible.

## Design summary

Here is a summary of how things are organized in maddy in general. It explains
things from the developer perspective and is meant to be used as an
introduction by the new developers/contributors. It is recommended to read
user documentation to understand how things work from the user perspective as
well.

- User documentation: [maddy.conf(5)](maddy.conf.5.scd)
- Design rationale: [Comments on design (Wiki)](https://github.com/foxcpp/maddy/wiki/Dev:-Comments-on-design)

There are components called "modules". They are represented by objects
implementing the module.Module interface. Each module gets its unique name.
The function used to create a module instance is saved with this name as a key
into the global map called "modules registry". Each module can be created
multiple times, each instance gets its unique name and is placed into a global
map (a separate one) too.

Modules can reference each other by instance names (module.GetInstance). When a
module instance reference is acquired, the caller usually checks whether the
module in question implements the needed interface. Module implementers are
discouraged from using module.GetInstance directly and should prefer using
ModuleFromNode or config.Map matchers. These functions handle "inline module
definition" syntax in addition to simple instance references.

Module instances are initialized lazily if they are referenced by other modules
(module.GetInstance calls mod.Init if necessary). Module instances not
referenced explicitly anywhere but still defined in the configuration are
initialized in arbitrary order by the Start function (below).

Module instances that are defined by the code itself ("implicitly defined") may
be left uninitialized unless they are used.

A single module instance can have one or more names. The first name is called
"instance name" and is the primary one, it is used in logs.  Other names are
called "aliases" and only used by module.GetInstance (e.g. module instance can
be fetched by any name).

Some modules attach additional meaning to names. This is generally accepted
since it is better to have only a single instance managing one resource. For
example, module instance implementing forwarding to the upstream server can not
reasonably enforce any limitations unless it is only one instance "controlling"
that upstream. Unique names requirement helps a lot here.

"Semantical names" idea explained above is not applied when modules instances
are defined "inline" (in place they are used in). These instances have no
instance names and are not added to the global map so they can not be accessed
by modules other than one that used ConfigFromNode on corresponding config
block. All arguments after the module name in an inline definition represent
"inline arguments". They are passed to the module instance directly and not
used anyhow by other code (i.e. they are not guaranteed to be unique).

