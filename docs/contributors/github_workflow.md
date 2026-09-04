<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# GitHub Workflow

This guide assumes you have already cloned the upstream repo to your system via `git clone`, or via `go get github.com/k0sproject/k0s`.

## Fork The Project

1. Go to http://github.com/k0sproject/k0s
2. On the top, right-hand side, click on "fork" and select your username for the fork destination.

## Adding the Forked Remote

```shell
export GITHUB_USER={ your github username }
```

```shell
cd $WORKDIR/k0s
git remote add $GITHUB_USER git@github.com:${GITHUB_USER}/k0s.git

# Prevent push to Upstream
git remote set-url --push origin no_push

# Set your fork remote as a default push target
git push --set-upstream $GITHUB_USER main
```

Your remotes should look something like this:

```shell
git remote -v
```

```shell
origin  https://github.com/k0sproject/k0s (fetch)
origin  no_push (push)
my_fork git@github.com:{ github_username }/k0s.git (fetch)
my_fork git@github.com:{ github_username }/k0s.git (push)
```

## Create & Rebase Your Feature Branch

Create a feature branch and switch to it:

```shell
git checkout -b my_feature_branch
```

Rebase your branch:

```shell
git fetch origin && \
  git rebase origin/main
```

```shell
Current branch my_feature_branch is up to date.
```

Please don't use `git pull` instead of the above `fetch` / `rebase`. `git pull` does a merge, which leaves merge commits. These make the commit history messy and violate the principle that commits ought to be individually understandable and useful.

## Commit & Push

Commit and sign your changes:

```shell
git commit --signoff
```

Commit messages should have a [short](https://cbea.ms/git-commit/#limit-50),
[capitalized](https://cbea.ms/git-commit/#capitalize) subject [without trailing
period](https://cbea.ms/git-commit/#end) as first line. After the subject a
[blank line](https://cbea.ms/git-commit/#separate) and then a longer description
that [explains why](https://cbea.ms/git-commit/#why-not-how) the change was
made, unless it is obvious.

Use [imperative mood](https://cbea.ms/git-commit/#imperative) in the commit message.

For example:

```text
Summarize changes in around 50 characters or less

More detailed explanatory text, if necessary. Wrap it to about 72
characters or so. In some contexts, the first line is treated as the
subject of the commit and the rest of the text as the body. The
blank line separating the summary from the body is critical (unless
you omit the body entirely); various tools like `log`, `shortlog`
and `rebase` can get confused if you run the two together.

Explain the problem that this commit is solving. Focus on why you
are making this change as opposed to how (the code explains that).
Are there side effects or other unintuitive consequences of this
change? Here's the place to explain them.

Further paragraphs come after blank lines.

 - Bullet points are okay, too

 - Typically a hyphen or asterisk is used for the bullet, preceded
   by a single space, with blank lines in between.

Put references to earlier commits and other relevant material at the
bottom, like this:

Fixes: a50271db8 ("Implement k0s feature gates")
See: f9773809e ("Encapsulate debug flag handling")
See: https://kubernetes.io/docs/reference/command-line-tools-reference/feature-gates/

Signed-off-by: Name Lastname <user@example.com>
```

The commit message is read by whoever looks at this change years from now via
`git blame` or `git log`. Write it for them:

- State what was wrong, what the change does, and why. If the change fixes a
  regression, say what the effect was in released versions.
- Mention non-obvious constraints the code depends on, such as ordering,
  timing, or compatibility with data written by earlier versions. These are
  the things a future reader can't reconstruct from the diff.
- If you remove dead code or do unrelated cleanup while you're there, say so.
  Don't present it as part of the fix.
- Leave out how you found the bug, what you tried, what the change does *not*
  fix, and anything addressed to the reviewer. That material belongs in the
  pull request description or the discussion.
- Don't reference source file paths unless it aids understanding. The diff
  already lists them.

Use `Fixes:` for the commit that introduced the problem this change corrects,
if it can be identified. The issue a change resolves is referenced from the
pull request description instead, where GitHub's closing keywords act on it.
Use `See:` for anything else that helps to understand the change: related
commits, external documentation, specifications, and the like.

When referencing other commits, use the abbreviated hash, e.g. as printed by
`git log --oneline`, along with the subject line: `a50271db8 ("Implement k0s
feature gates")`. That way, the reference is meaningful in `git log`, whether or
not the hash resolves. In general, prefer what works locally over what only
works on GitHub.

Note that GitHub adds a timeline event to an issue or pull request whenever a
commit referencing it is pushed to any public repository, including your own
fork. Since a rebase creates new commits, every force-push repeats those events,
and frequent rebases leave a trail of identical entries in the timeline. That's
another reason to reference issues and pull requests from the pull request
description rather than from commit messages.

When ready, push your changes to your fork's repository:

```shell
git push --set-upstream my_fork my_feature_branch
```

You can go back and edit/build/test some more, then `commit --amend` in a few
cycles.

After review rounds, fold any fixup commits into the commits they belong to and
rewrite the messages so that each commit describes its final change, not its
history. Commits titled "Address review comments" are fine during reviews, but
clutter the history once merged. See [Squashing Commits](#squashing-commits) for
how to do that.

## Open a Pull Request

See GitHub's docs on how to [create a pull request from a fork][pr-from-fork].

The purpose of the pull request description is to facilitate discussion. It
should include the context, the alternatives considered, what was verified and
how it was verified, and what the change deliberately leaves out. It must not
contradict the commit messages. When you rewrite commits after a review round,
re-read the description and update it, too.

[pr-from-fork]: https://docs.github.com/pull-requests/how-tos/create-pull-requests/creating-a-pull-request-from-a-fork

### Get a code review

Once your pull request has been opened it will be assigned to one or more
reviewers, and will go through a series of [smoke tests].

Commit changes made in response to review comments should be added to the same
branch on your fork.

Very small PRs are easy to review. Very large PRs are very difficult to review.

[smoke tests]: testing.md#integration-tests-aka-inttests-or-smoketests

### Squashing Commits

Commits on your branch should represent meaningful milestones or units of work.
Small commits that contain typo fixes, rebases, review feedbacks, etc should be squashed.

To do that, it's best to perform an [interactive rebase](https://git-scm.com/book/en/v2/Git-Tools-Rewriting-History):

#### Example

Rebase your feature branch against upstream main branch:

```shell
git rebase -i origin/main
```

If your PR has 3 commits, output would be similar to this:

```shell
pick f7f3f6d Changed some code
pick 310154e fixed some typos
pick a5f4a0d made some review changes

# Rebase 710f0f8..a5f4a0d onto 710f0f8
#
# Commands:
# p, pick <commit> = use commit
# r, reword <commit> = use commit, but edit the commit message
# e, edit <commit> = use commit, but stop for amending
# s, squash <commit> = use commit, but meld into previous commit
# f, fixup <commit> = like "squash", but discard this commit's log message
# x, exec <command> = run command (the rest of the line) using shell
# b, break = stop here (continue rebase later with 'git rebase --continue')
# d, drop <commit> = remove commit
# l, label <label> = label current HEAD with a name
# t, reset <label> = reset HEAD to a label
# m, merge [-C <commit> | -c <commit>] <label> [# <oneline>]
# .       create a merge commit using the original merge commit's
# .       message (or the oneline, if no original merge commit was
# .       specified). Use -c <commit> to reword the commit message.
#
# These lines can be re-ordered; they are executed from top to bottom.
#
# However, if you remove everything, the rebase will be aborted.
#
# Note that empty commits are commented out
```

Use a command line text editor to change the word `pick` to `f` of `fixup` for the commits you want to squash, then save your changes and continue the rebase:

Per the output above, you can see that:

```shell
fixup <commit> = like "squash", but discard this commit's log message
```

Which means that when rebased, the commit message "fixed some typos" will be removed, and squashed with the parent commit.

### Push Your Final Changes

Once done, you can push the final commits to your branch:

```shell
git push --force
```

You can run multiple iteration of `rebase`/`push -f`, if needed.
