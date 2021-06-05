# Contributing Guidelines

Of course, we love our contributors. Thanks for spending time on making maddy
better.

## Reporting bugs

**Issue tracker is meant to be used only if you have a problem or a feature
request. If you just have some questions about maddy - prefer to use the
[IRC channel](https://webchat.oftc.net/?channels=maddy&uio=MT11bmRlZmluZWQb1).**

- Provide log files, preferably with 'debug' directive set.
- Provide the exact steps to reproduce the issue.
- Provide the example message that causes the error, if applicable.
- "Too much information is better than not enough information".

Issues without enough information will be ignored and possibly closed.
Take some time to be more useful.

See SECURITY.md for information on how to report vulnerabilities.

## Contributing Code

0. Use common sense.
1. Learn Git. Especially, what is `git rebase`. We may ask you to use it if
   needed.
2. Tell us that you are willing to work on an issue.
3. Fork the repo. Create a new branch based on `dev`, write your code. Open a
   PR.

Ask for advice if you are not sure. We don't bite.

maddy design summary and some recommendations are provided in
[HACKING.md](../HACKING.md) file.

## Commits

1. Prefix commit message with a package path if it affects only a single
   package. Omit `internal/` for brevity.
2. Provide reasoning for details in the source code itself (via comments),
   provide reasoning for high-level decisions in the commit message.
3. Make sure every commit builds & passes tests. Otherwise `git bisect` becomes
   unusable.

## Git workflow

`dev` branch includes the in-development version for the next feature release.
It is based on commit of the latest stable release and is merged into `master`
on release via fast-forward. Unlike `master`, `dev` **is not a protected branch
and may get force-pushes**.

`master` branch contains the latest stable release and is frozen between
releases.

`fix-X.Y` are temporary branches containing backported security fixes.
They are based on the commit of the corresponding stable release and exist
while the corresponding release is maintained. A `fix-*` branch is not created
for the latest release. Changes are added to these branches by cherry-picking
needed commits from the `dev` branch.
