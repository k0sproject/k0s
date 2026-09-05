<!--
SPDX-FileCopyrightText: 2026 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Code guidelines

This page collects guidelines for the k0s code base. Many of them stem from
things that come up frequently during code reviews, or from k0s design
constraints, such as being a per-node agent. These guidelines complement
linters, which enforce general formatting and Go coding style. However, linters
perform static code analysis and can't judge broader concepts, such as whether a
change is the right size or if a comment is appropriate and accurate.

None of this is set in stone. The right trade-off depends on what a particular
change does, and reviewers may weigh things differently from case to case. The
guidelines also evolve with the project: if one of them stops serving its
purpose, change it rather than working around it.

Existing code doesn't follow all of this. When touching such code, prefer these
guidelines over consistency with the surrounding code, but don't churn unrelated
lines just to bring them in line.

## Size the code to the problem

Match the amount of structure to what the change needs. A few lines at the call
site don't need a helper, a helper doesn't need to be exported, and neither
needs its own abstraction until there's a second user. Extra structure isn't
free; it creates more surface area to read, review, and maintain.

Remove dead code when you come across it, but keep such cleanups separate from
functional changes, at least in the [commit message].

## Comments and names

Comments cost readers time every time they read the code. Keep them crisp. Don't
comment on obvious things or things that readers can quickly figure out. Comment
on what they can't, such as ordering requirements, timing assumptions, or
compatibility with earlier versions. Judge a comment based on what it adds, not
by its length. Restating what the code already says is unhelpful even in a
single line, whereas a comment that explains an important, broader concept which
can't be better documented elsewhere may span several sentences or even
paragraphs.

Comments describe the present, not the past. Version control holds the history.
A comment that explains what the code used to do, which bug motivated a test, or
how a change came about belongs in the [commit message], not in the code. The
same goes for names. Name things for what they are, not for how they relate to a
previous version of the code or to a change in progress.

Avoid referencing file names or line numbers in comments. They become outdated
with the next refactoring, and nothing will tell you. Refer to Go identifiers
instead, using [doc links] where possible, so that the reference stays navigable
and tooling catches renames.

[doc links]: https://go.dev/doc/comment#doclinks

## Errors

An error message is a fragment that gets chained with others through wrapping.
Keep it lowercase and without trailing punctuation, as per the [Go
conventions][error-strings], so that `failed to apply stack: cannot create
resource: connection refused` reads as one line.

Add context to an error only if it's neither part of the error already nor known
to the caller anyway. Many errors, such as those returned by the standard
library's I/O functions, are self-describing, and the caller knows which
function it called. Wrapping them with a message that restates the operation
adds noise, not information.

[error-strings]: https://go.dev/wiki/CodeReviewComments#error-strings

## Logging

Logs are the primary tool users have to understand what k0s is doing on their
systems. They're often read out of context, in between lots of other output, by
someone who doesn't know the code. A good log message tells them what happened
and, ideally, what they can do about it, such as which file, resource, or
configuration to examine. That helps both sides. Users who can troubleshoot
their own systems don't need to ask for help, and the questions that do reach
the developers arrive with the right information attached.

Log what helps readers understand what's going on and why. What's noise in one
situation is a decisive hint in another, and the log level is what separates the
two. It decides who sees a message, and when.

For instance, a periodic task that finds nothing is noise at `Info` level, but
useful at `Debug` level. Pick the level based on what the reader is expected to
do with the message. As a rule of thumb, use `Debug` for details that only
matter when troubleshooting, `Info` for the normal course of events, `Warn` for
problems that k0s handles on its own, but that may need attention if they
persist, and `Error` for failures that require someone to act.

While retries are ongoing, log each failed attempt at `Debug` if failures are
expected or at `Warn` if they are not. Escalate to `Error` once retries are
exhausted or if the operation has not succeeded within a reasonable time. If the
consequences aren't obvious, explain what will happen, e.g. if an operation will
be retried, skipped, or abandoned.

Prefer putting variable data, especially error values, into fields rather than
into the message text, so that messages stay stable and searchable. Fields are
useful when the same name is used across messages because a filter on that name
finds every message about the same thing. Messages without placeholders work the
other way around. They read the same wherever they're logged, so filtering on
them finds all things affected by the same event. Values that no other message
would share are fine in the message text. Keep messages in sync with the
conditions they describe. When a condition changes, its message usually has to
change, too.

Unlike an error message, a log message stands on its own. Write it as a
sentence, starting with a capital letter and without a trailing period.

Log errors where they're handled, not where they're passed on. An error that's
both logged and returned will usually end up in the logs once for every layer it
passes through. When logging an error, avoid repeating what the error message
already says. The message provides context, while the error provides the cause.

## Compatibility across versions

k0s is a per-node agent. Each node reads its own configuration at startup and
acts locally. Cluster-wide coordination is the responsibility of the users or of
additional tooling, such as [Autopilot], [k0sctl] or [k0smotron]. Nodes are
upgraded individually rather than all at once, so controllers and workers of
adjacent versions run side by side for a while, within the limits of the
[version skew policy].

Any change has to work under those conditions: a new version must interoperate
with the previous one, and it must keep reading whatever an earlier version has
persisted, on disk or in the cluster. The burden is on the newer version, since
the older one can't know anything about behavior that didn't exist yet. Whatever
it persists itself must either stay usable by the previous version, or be kept
apart from what the previous version reads, for example by versioning its name
or location.

Prefer designs that converge node by node to those that require every node to
flip simultaneously. If the old and new behaviors can't coexist, don't hide the
switch in the upgrade. Keep the old behavior until users explicitly enable the
new one so that they can choose when the disruption happens, and document what
the switch entails.

Changed configuration defaults must not alter the behavior of clusters that were
created with a previous version. New clusters get the new defaults, while
existing clusters keep behaving as before unless they opt in. This applies to
defaults that stand in for a choice users could have made in the configuration,
not to properties of the release itself, such as the versions of the components
that k0s ships.

### Transitional code

Code that only exists to migrate from an earlier version or clean up leftovers
will be removed eventually. Keep it small and local, and mark it with a `TODO`
note indicating the version in which it can be removed. Don't build abstractions
or test suites around something scheduled for deletion unless the logic is
complex enough to justify the test. Verify it once and explain how in the pull
request.

[Autopilot]: ../autopilot.md
[k0sctl]: ../k0sctl-install.md
[k0smotron]: https://github.com/k0sproject/k0smotron
[version skew policy]: ../version-skew-policy.md

## Tests

Use [testify] for assertions, and add assertion messages that state the
expectation, so that a failure reads as a sentence. Test names describe the
behavior under test, not the reason the test was added. Use obviously made-up
values for test inputs that aren't the subject of the test, so that it's clear
which values matter and which don't. See the [testing guide] for how to run the
different kinds of tests, and what's expected before a pull request is merged.

[testify]: https://github.com/stretchr/testify
[testing guide]: testing.md

[commit message]: github_workflow.md#commit-push
