# Contributing Guidelines

## Reporting bugs

**Issue tracker is meant to be used only if you have a concrete localized bug
to report or a feature request. If you just have some questions about maddy -
prefer to use the [IRC
channel](https://webchat.oftc.net/?channels=maddy&uio=MT11bmRlZmluZWQb1).**

- Provide log files, preferably with 'debug' directive set.
- Provide the exact steps to reproduce the issue.
- Provide the example message that causes the error, if applicable.
- "Too much information is better than not enough information".

Issues without enough information MAY be ignored and possibly closed without
further investigation.

See SECURITY.md for information on how to report vulnerabilities.

## Contributing Code

maddy design summary and some recommendations are provided in
[HACKING.md](../HACKING.md) file.

### Preparing work

All work SHOULD happen with an existing relevant GitHub Issue. If it doesn't
exist - you SHOULD open one.

Large features should undergo design review. This is indicated by GitHub Issue
having the `rfc` label. It means there is no consensus how the feature should
be implemented or whether it should be implemented at all. PRs for Issues with
`rfc` label WILL NOT be merged.

Following information should be provided and approved by a maintainer during
design review before any code work can be started:

**Product Level Design:**
1. Concrete use-cases for the feature: the problem that is solved by new feature.
2. Considered existing alternatives (optional): explain why you can't use
   existing features to solve the same problem.

**Technical Level Design:**
1. Implementation context: short description for existing functionality in code
   that will be extended or otherwise modified.
2. Implementation plan (free-form description): new entities, their contracts
   and interactions.
3. Considered alternative designs (optional): if design went through multiple
   iterations - provide short summary for previous iterations and different they were.
4. Out-of-scope work (optional): what the feature is NOT and it NOT intended to be.

If you are working a small feature (e.g. new configuration directive to toggle
existing behavior) or non-invasive bug fix you don't need to provide design
description.

## Git workflow

- Leave a comment on a Issue if you are working on it.
- Commit messages SHOULD reference relevant Issue number (`Fixes #NNN` is enough).
- Commit messages SHOULD follow [Conventional Commits](https://www.conventionalcommits.org/en/v1.0.0/) scheme.
- Within a single PR, commits history SHOULD follow [recipe-like
  structure](https://www.bitsnbites.eu/git-history-work-log-vs-recipe/) -
  prefer amending existing commits and force-pushing instead of adding new
  commits to fix issues found during review. This requirement is MUST for any
  PR exceeding 1000 LoC to simplify review process.

### Main branches

`dev` branch is the in-development version for the next feature release.
If you are working on a new feature - start from this branch.

`master` branch is the in-development version for the next bugfix release.
If you are working on a bugfix or a minor documentation/code improvement -
start from this branch.

`X.Y-fixes` are temporary branches containing backported security fixes.
They are based on the commit of the corresponding stable release and exist
while the corresponding release is maintained. A `fix-*` branch is not created
for the latest release. Changes are added to these branches by cherry-picking
needed commits from the `master` branch.

## LLM policy

There are additional restrictions if you use LLM agents to write code.

- LLM usage MUST be disclosed. Preferred way to do so is to add "Assisted-By:"
  line to commit message and/or PR description with agent and model name.
- PR description and review responses MUST be written by a human. PRs from fully
  autonomous agents will be rejected.
- The code author agrees to the [Developer Certificate of
  Origin](https://developercertificate.org/). In particular, the author
  certifies that they have the right to submit the patch under the project's
  open-source license. This is typically indicated by adding `Signed-off-by`
  trailer added to the commit description.
- LLMs tend to write overly verbose code and documentation. You SHOULD review
  and rewrite such sections by hand to improve clarity. If left unchanged -
  such changes MAY be rejected without further review.
