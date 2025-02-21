# Integration tests a.k.a e2e testing

This folder is the root of k0s integration tests. These tests are such that run the actual k0s clusters, currently using [bootloose](https://github.com/k0sproject/bootloose) as the target environment.

## Running the tests

There's a [`Makefile`](Makefile) defining the tests as a set of make targets.

## Test design

We're currently building the tests as Golang tests with the help of [testify](https://github.com/stretchr/testify/) test suite concept. The suite concept allows us to have suite level setup and teardown functionality so we can bootstrap and delete the test environment properly during testing. The suite setup phase creates the "infrastructure" for the tests and the teardown, as the name implies, deletes the infra.

## Keeping the test env after tests

Sometimes, especially when debugging some test failures, it's good to leave the environment running after the tests have ran. To control that behavior there's an env variable called `K0S_KEEP_AFTER_TESTS`. The value given to that has the following logic:

- no value or `K0S_KEEP_AFTER_TESTS="never"`: The test env is NOT left running regardless of the test results
- `K0S_KEEP_AFTER_TESTS="always"`: The test env is left running regardless of the test results
- `K0S_KEEP_AFTER_TESTS="failure"`: The test env is left running only if the tests have failed

The test output show how to run manual cleanup for the environment, something like:

```shell
TestNetworkSuite: bootloosesuite.go:138: bootloose cluster left intact for debugging. Needs to be manually cleaned with: bootloose delete --config /tmp/afghzzvp-bootloose.yaml
```

This allows you to run manual cleanup after you've done the needed debugging.

## Which tests to run when?

The plan is to run only the basic (and quick) smoke tests on each PR commit. We should build some bot like functionality to run longer and more expensive tests using some trigger before the final merge of a PR. Naturally we should be able to run any of the tests locally, or at least triggered locally, to ensure we can actually debug what is happening.
