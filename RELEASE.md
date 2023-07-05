# Releasing k0s

We try to follow the practice of releasing often. That allows us to have smaller releases and thus, hopefully, we'll also break less things while doing so.

## Creating a release

Creating a release happens via GitHub Actions by creating an [annotated](https://git-scm.com/book/en/v2/Git-Basics-Tagging#_creating_tags) git tag.
To create the annotated tag do:

```git tag -a -m "release <version>" <version>```

When releasing multiple versions make sure to tag from the oldest version to the newest in order, this is important because it will affect the order in the release page.

**WARNING**: The tag cannot be pushed with `git push --tags` because it won't trigger the [release GitHub Action](https://github.com/k0sproject/k0s/actions/workflows/release.yml). You must do `git push <tag>`.

Tag creation triggers the release workflow which will do most of the heavy-lifting:

- Create the actual release in [releases](https://github.com/k0sproject/k0s/releases/)
- Build `k0s` binary on both AMD64 and ARM64 architectures
- Push the bins into the release as downloadable assets

After the action completes, the release will be in `draft` state to allow manual modification of the release notes. Currently there is no automation for the release notes, this has to be manually collected.

Once the release notes are done we can publish the release.

If for some reason there is an error triggering the action, it is safe to delete the tag remotely with `git push --delete origin <tag>` and push it again.

The above steps are encapsulated in the [`hack/release.sh`](hack/release.sh)
shell script for convenience.

## Semver

We're following [semantic versioning](https://semver.org/) for version numbering. Currently we're working on 0.y.z series so the rules are interpreted in bit more relaxed way.

## Betas, RCs and others

We usually bake couple beta or RC releases before pushing out the final release. This allows us to run final verifications on the system with a package that's build exactly the same as a real release.

One of the steps involved in final testing stages for a release is to run [Kubernetes conformance](https://github.com/cncf/k8s-conformance) tests for a RC build. For more info how to run conformance testing for k0s read [this](docs/conformance-testing.md).
