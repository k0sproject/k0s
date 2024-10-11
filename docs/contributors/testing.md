# Testing Your Code

k0s uses github actions to run automated tests on any PR, before merging.
However, a PR will not be reviewed before all tests are green, so to save time and prevent your PR from going stale, it is best to test it before submitting the PR.

## Run Local Verifications

Please run the following style and formatting commands and fix/check-in any changes:

1. Linting

   We use [`golangci-lint`](https://golangci-lint.run/) for style verification.
   In the repository's root directory, simply run:

   ```shell
   make lint
   ```

   There's no need to install `golangci-lint` manually. The build system will
   take care of that.

2. Go fmt

   ```shell
   go fmt ./...
   ```

3. Checking the documentation

   Verify any changes to the documentation by following the instructions
   [here](docs.md#testing-docs-locally).

4. Pre-submit Flight Checks

   In the repository root directory, make sure that:

   * `make build && git diff --exit-code` runs successfully.  
     Verifies that the build is working and that the generated source code
     matches the one that's checked into source control.
   * `make check-unit` runs successfully.  
     Verifies that all the unit tests pass.
   * `make check-basic` runs successfully.  
     Verifies basic cluster functionality using one controller and two workers.
   * `make check-hacontrolplane` runs successfully.  
     Verifies that joining of controllers works.

   Please note that this last test is prone to "flakiness", so it might fail on
   occasion. If it fails constantly, take a deeper look at your code to find the
   source of the problem.

   If you find that all tests passed, you may open a pull request upstream.

## Opening A Pull Request

### Draft Mode

You may open a pull request in [draft mode](https://github.blog/2019-02-14-introducing-draft-pull-requests).
All automated tests will still run against the PR, but the PR will not be assigned for review.
Once a PR is ready for review, transition it from Draft mode, and code owners will be notified.

### Conformance Testing

Once a PR has been reviewed and all other tests have passed, a code owner will run a full end-to-end conformance test against the PR. This is usually the last step before merging.

### Pre-Requisites for PR Merge

In order for a PR to be merged, the following conditions should exist:

1. The PR has passed all the automated tests (style, build & conformance tests).
2. PR commits have been signed with the `--signoff` option.
3. PR was reviewed and approved by a code owner.
4. PR is rebased against upstream's main branch.

## Cleanup the local workspace

In order to clean up the local workspace, run `make clean`. It will clean up all
of the intermediate files and directories created during the k0s build. Note
that you can't just use `git clean -X` or even `rm -rf`, since the Go modules
cache sets all of its subdirectories to read-only. If you get in trouble while
trying to delete your local workspace, try `chmod -R u+w /path/to/workspace &&
rm -rf /path/to/workspace`.
