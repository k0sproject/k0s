<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Testing Your Code

k0s uses GitHub Actions to run automated tests on any pull request before
merging it. However, your PR will not be reviewed until all tests are green. To
save time and prevent your PR from going stale, it is best to run the tests
before submitting the PR.

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

   Follow the [instructions for testing the documentation
   locally][testing-docs-locally] to verify any changes.

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

[testing-docs-locally]: docs.md#testing-docs-locally

## Integration tests (a.k.a. inttests or smoketests)

The integration tests are located in the inttest directory and are run in CI as
"smoketests". These tests use Docker and [bootloose-based][bootloose] nodes to
launch k0s clusters locally.

[bootloose]: https://github.com/k0sproject/bootloose

### Requirements

* Docker (Linux) with the ability to run privileged containers.
* Sufficient disk space for the bootloose images and test artifacts.

### Running locally

You can run a single integration test by calling `make` from the repository
root:

```shell
make check-<name>
```

Examples:

```shell
make check-basic
make check-ap-airgap
```

Some tests, such as those related to airgap and IPv6, require image bundles. The
Makefile automatically builds and integrates those when you run `make
check-...`. The underlying environment variables are:

* `K0S_IMAGES_BUNDLE` (airgap bundle)
* `K0S_EXTRA_IMAGES_BUNDLE` (IPv6 bundle)

### Debugging local failures

When a test fails, the suite writes logs and a [support bundle] to the temporary
directory (as determined by [os.TempDir]), e.g.:

* `/tmp/controller*.out.log`, `/tmp/controller*.err.log`
* `/tmp/worker*.out.log`, `/tmp/worker*.err.log`
* `/tmp/support-bundle.tar.gz`

Start by inspecting the logs. You can also use [sbctl] with the support bundle
to inspect the state of the test cluster at the end of the test execution.

[support bundle]: https://github.com/replicatedhq/troubleshoot?tab=readme-ov-file#support-bundle
[os.TempDir]: https://pkg.go.dev/os#TempDir
[sbctl]: https://github.com/replicatedhq/sbctl?tab=readme-ov-file#command-line-tool-for-examining-k8s-resources-in-troubleshoots-support-bundles

### Debugging CI failures

When a CI run fails, the aforementioned files are uploaded and attached as job
artifacts. In the GitHub Actions UI, open the failed job, download and extract
the artifact for that test (named `<smoketest-name>-<arch>-files` or similar)
and extract it. You can then use the extracted files in the same way described
in the previous section.

### Running CI on your fork

The k0s GitHub Action workflows use GitHub's hosted runners whenever possible.
This allows you to run the same CI workflows on your fork by pushing to the main
branch or opening a pull request on your fork. Ensure that GitHub Actions are
enabled for the forked repository. Note that forked repositories do not have
access to k0s's ARMv7 runners; therefore, all ARMv7-related workflow runs will
be skipped.

Running the CI on your fork may provide faster feedback than running it on a
pull request against the k0s repository because running the GitHub workflows for
pull requests requires approval from a k0s maintainer. Additionally, it allows
you to tinker with the tests and workflows however you like to debug things.
This could be useful if you cannot run the smoketests locally.

## Opening A Pull Request

### Draft Mode

You may open a pull request in [draft mode](https://github.blog/2019-02-14-introducing-draft-pull-requests).
All automated tests will still run against the PR, but the PR will not be assigned for review.
Once a PR is ready for review, transition it from Draft mode, and code owners will be notified.

### Pre-Requisites for PR Merge

In order for a PR to be merged, the following conditions should exist:

1. The PR has passed all the automated tests (style, build & tests).
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
