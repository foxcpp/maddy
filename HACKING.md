## Design goals

- **Make it easy to deploy.**
  Minimal configuration changes should be required to get a typical mail server
  running. Though, it is important to avoid making guesses for a
  "zero-configuration". A wrong guess is worse than no guess.

- **Provide 80% of needed components.**
  E-mail has evolved into a huge mess. With a single package to do one thing, it
  quickly turns into a maintenance nightmare. Put all stuff mail server
  typically needs into a single package. Though, leave controversial or highly
  opinionated stuff out, don't force people to do things our way
  (see next point).

- **Interoperate with existing software.**
  Implement (de-facto) standard protocols not only for clients but also for
  various server-side helper software (content filters, etc).

- **Be secure but interoperable.**
  Verify DKIM signatures by default, use DMRAC policies by default, etc. This
  makes default setup as secure as possible while maintaining reasonable
  interoperability. Though, users can configure maddy to be stricter.

- **Achieve flexibility through composability.**
  Allow connecting components in arbitrary ways instead of restricting users to
  predefined templates.

- **Use Go concurrency features to the full extent.**
  Do as much I/O as possible in parallel to minimize latencies. It is silly to
  not do so when it is possible.

## Design summary

Here is a summary of how things are organized in maddy in general. It explains
things from the developer perspective and is meant to be used as an
introduction by the new developers/contributors. It is recommended to read
user documentation to understand how things work from the user perspective as
well.

- User documentation: [maddy.conf(5)](docs/man/maddy.5.scd)
- Design rationale: [Comments on design (Wiki)][1]

There are components called "modules". They are represented by objects
implementing the module.Module interface. Each module gets its unique name.
The function used to create a module instance is saved with this name as a key
into the global map called "modules registry".

Whenever module needs another module for some functionality, it references it
using a configuration directive with a matcher that internally calls
`modconfig.ModuleFromNode`. That function looks up the module "constructor" in
the registry, calls it with corresponding arguments, checks whether the
returned module satisfies the needed interfaces and then initializes it.

Alternatively, if configuration uses &-syntax to reference existing
configuration block, `ModuleFromNode` simply looks it up in the global instances
registry. All modules defined the configuration as a separate top-level blocks
are created before main initialization and are placed in the instances registry
where they can be looked up as mentioned before.

Top-level defined module instances are initialized (`Init` method) lazily as
they are required by other modules. 'smtp' and 'imap' modules follow a special
initialization path, so they are always initialized directly.

## Error handling

Familiarize yourself with the `github.com/foxcpp/maddy/framework/exterrors`
package and make sure you have the following for returned errors:
- SMTP status information (smtp\_code, smtp\_enchcode, smtp\_msg fields)
  - SMTP message text should contain a generic description of the error
    condition without any details to prevent accidental disclosure of the
    server configuration details.
- `Temporary() == true` for temporary errors (see `exterrors.WithTemporary`)
- Field that includes the module name

The easiest way to get all of these is to use `exterrors.SMTPError`.
Put the original error into the `Err` field, so it can be inspected using
`errors.Is`, `errors.Unwrap`, etc. Put the module name into `CheckName` or
`TargetName`. Add any additional context information using the `Misc` field.
Note, the SMTP status code overrides the result of `exterrors.IsTemporary()`
for that error object, so set it using `exterrors.SMTPCode` that uses
`IsTemporary` to select between two codes.

If the error you are wrapping contains details in its structure fields (like
`*net.OpError`) - copy these values into `Misc` map, put the underlying error
object (`net.OpError.Err`, for example) into the `Err` field.
Avoid using `Reason` unless you are sure you can provide the error message
better than the `Err.Error()` or `Err` is `nil`.

Do not attempt to add a SMTP status information for every single possible
error. Use `exterrors.WithFields` with basic information for errors you don't
expect. The SMTP client will get the "Internal server error" message and this
is generally the right thing to do on unexpected errors.

### Goroutines and panics

If you start any goroutines - make sure to catch panics to make sure severe
bugs will not bring the whole server down.

## Adding a check

"Check" is a module that inspects the message and flags it as spam or rejects
it altogether based on some condition.

The skeleton for the stateful check module can be found in
`internal/check/skeleton.go`.  Throw it into a file in
`internal/check/check_name` directory and start ~~breaking~~ extending it.

If you don't need any per-message state, you can use `StatelessCheck` wrapper.
See `check/dns` directory for a working example.

Here are some guidelines to make sure your check works well:
- RTFM, docs will tell you about any caveats.
- Don't share any state _between_ messages, your code will be executed in
  parallel.
- Use `github.com/foxcpp/maddy/check.FailAction` to select behavior on check
  failures. See other checks for examples on how to use it.
- You can assume that order of check functions execution is as follows:
  `CheckConnection`, `CheckSender`, `CheckRcpt`, `CheckBody`.

## Adding a modifier

"Modifier" is a module that can modify some parts of the message data.

Note, currently this is not possible to modify the body contents, only header
can be modified.

Structure of the modifier implementation is similar to the structure of check
implementation, check `modify/replace\_addr.go` for a working example.

[1]: https://github.com/foxcpp/maddy/wiki/Dev:-Comments-on-design
