# Contributing Guidelines

Of course, we love our contributors. Thanks for spending time on making maddy
better.

## Reporting bugs

**Issue tracker is meant to be used only if you have a problem or a feature
request. If you just have some questions about maddy - prefer to use the
[IRC channel](https://webchat.freenode.net/?channels=%23%23maddy).**

- Provide log files, preferably with 'debug' directive set.
- Provide the exact steps to reproduce the issue.
- Provide the example message that causes the error, if applicable.
- "Too much information is better than not enough information".

Issues without enough information will be ignored and possibly closed.
Take some time to be more useful.

See SECURITY.md for information on how to report vulnerabilities.

## Contributing Code

0. Use common sense.
1. Learn Git. Especially, what is `git rebase`. We may ask you to use it if needed.
2. Tell us that you are willing to work on an issue.
3. Fork the repo. Create a new branch, write your code. Open a PR.

Ask for advise if you are not sure. We don't bite.

maddy design summary and some recommendations are provided in
[HACKING.md](../HACKING.md) file.

## Commits

1. Prefix commit message with a package path if it affects only a single
   package.
2. Provide reasoning for details in the source code itself (via comments),
   provide reasoning for high-level decisions in the commit message.
3. Make sure every commit builds & passes tests. Otherwise `git bisect` becomes
   unusable.
