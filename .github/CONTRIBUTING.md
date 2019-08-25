# Contributing Guidelines

Of course, we love our contributors. Thanks for spending time on making maddy
better.

## Reporting bugs

**Issue tracker is meant to be used only if you have a problem or a feature
request. If you just have some questions about maddy - prefer to use the [IRC channel](https://webchat.freenode.net/?channels=%23%23maddy).**

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

Some hints regarding code design we would like to see:
- Generate a **small amount** of **useful** log messages.
- Generate **verbose** log messages with **'debug'** directive set.
- Create "modules" for **big** chunks of **reusable** functionality that may have
  **swappable implementations**.
- Ask for advise regarding design if you are not sure. We don't bite.
