maddy source tree
------------------

Main maddy code base lives here. No packages are intended to be used in
third-party software hence API is not stable.

Subdirectories are organized as follows:
```
/
  auxiliary libraries
endpoint/
  modules - protocol listeners (e.g. SMTP server, etc)
target/
  modules - final delivery targets (including outbound delivery, such as
  target.smtp, remote)
auth/
  modules - authentication providers
check/
  modules - message checkers (module.Check)
modify/
  modules - message modifiers (module.Modifier)
storage/
  modules - local messages storage implementations (module.Storage)
```
