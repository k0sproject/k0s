# Releasing k0s

We try to follow the practice of releasing often. That allows us to have smaller releases and thus, hopefully, we'll also break less things while doing so.

## Creating a release

Creating a release happens via Github Actions by creating a git tag. Tag creation triggers the release workflow which will do most of the heavy-lifting:

- Create the actual release in [releases](https://github.com/k0sproject/k0s/releases/)
- Build `k0s` binary on both AMD64 and ARM64 architectures
- Push the bins into the release as downloadable assets

After the action completes, the release will be in `draft` state to allow manual modification of the release notes. Currently there is no automation for the release notes, this has to be manually collected.

Once the release notes are done we can publish the release.

## Semver

We're following [semantic versioning](https://semver.org/) for version numbering. Currently we're working on 0.y.z series so the rules are interpreted in bit more relaxed way.

## Betas, RCs and others

We usually bake couple beta or RC releases before pushing out the final release. This allows us to run final verifications on the system with a package that's build exactly the same as a real release.

One of the steps involved in final testing stages for a release is to run [Kubernetes conformance](https://github.com/cncf/k8s-conformance) tests for a RC build. For more info how to run conformance testing for k0s read [this](docs/conformance-testing.md).
