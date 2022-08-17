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

The commit message should have a [short](https://cbea.ms/git-commit/#limit-50), [capitalized](https://cbea.ms/git-commit/#capitalize) title [without trailing period](https://cbea.ms/git-commit/#end) as first line. After the title
a [blank line](https://cbea.ms/git-commit/#separate) and then a longer description that [explains why](https://cbea.ms/git-commit/#why-not-how) the change was made, unless it is obvious.

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

If you use an issue tracker, put references to them at the bottom,
like this:

Fixes: https://github.com/k0sproject/k0s/issues/373
See also: #456, #789

Signed-off-by: Name Lastname <user@example.com>

```

You can go back and edit/build/test some more, then `commit --amend` in a few cycles.

When ready, push your changes to your fork's repository:

```shell
git push --set-upstream my_fork my_feature_branch
```

## Open a Pull Request

See GitHub's docs on how to [create a pull request from a fork][pr-from-fork].

[pr-from-fork]: https://docs.github.com/pull-requests/collaborating-with-pull-requests/proposing-changes-to-your-work-with-pull-requests/creating-a-pull-request-from-a-fork

### Get a code review

Once your pull request has been opened it will be assigned to one or more reviewers, and will go through a series of smoke tests.

Commit changes made in response to review comments should be added to the same branch on your fork.

Very small PRs are easy to review. Very large PRs are very difficult to review.

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
