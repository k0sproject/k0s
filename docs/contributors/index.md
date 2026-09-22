<!--
SPDX-FileCopyrightText: 2020 k0s authors
SPDX-License-Identifier: CC-BY-SA-4.0
-->

# Contributing to k0s

Thank you for taking the time to contribute to k0s! This page gives you an
overview of how to contribute, and where to find the details.

Please follow the **[k0sproject community's code of conduct][coc]** in all your
interactions with the project.

If you have found a security vulnerability, don't disclose it publicly. Report
it confidentially, as described in the **[security policy]**.

[coc]: https://github.com/k0sproject/community/blob/main/CODE_OF_CONDUCT.md
[security policy]: https://github.com/k0sproject/k0s/blob/main/SECURITY.md

## Contributing to the repository

K0s is more than its code. Contributions to the documentation are as welcome and
as valuable as contributions to the code base.

### Seek early feedback

If there's more than one way to approach your change, reach out via [GitHub
issues] or other [community channels](../README.md#join-the-community) before
you start building it. Alternatives to a feature and different implementation
paths are easier to compare in the abstract than in a finished diff. Discussing
early also shows whether a change fits the project's direction, and whether
someone is already working on it.

### Code

The [code guidelines](code_guidelines.md) collect the conventions of the k0s
code base beyond what the linters enforce, such as comments, error handling,
logging, and how to maintain compatibility across versions.

If your change affects how k0s is used or configured, update the documentation
along with it.

The [testing](testing.md) page explains how to run the linters, unit tests, and
integration tests locally, how to debug CI failures, and how to run CI on your
own fork.

### Documentation

The documentation lives under the `docs` directory and is published to
<https://docs.k0sproject.io>. Fixing a typo, clarifying a confusing section, or
writing a guide for something that isn't covered yet are all great
contributions.

The [documentation](docs.md) page explains how the docs are built, how to add
a page to the navigation, and how to preview your changes locally.

### Pull requests

All changes, to the documentation and to the code alike, arrive as pull
requests. The [GitHub workflow](github_workflow.md) page walks you through the
process of

- forking the repository,
- writing commits that serve one purpose each and are signed off to certify
  the [Developer Certificate of Origin][DCO],
- opening the pull request,
- and reworking your commits after a review round.

Before opening a pull request, run the linters and related tests locally, as
described on the [testing](testing.md) page. CI runs on a pull request only
after a maintainer has approved it, so catching problems locally saves a round
trip.

[DCO]: https://developercertificate.org/

## Contributing beyond the repository

Not every contribution is a change to this repository. There are many ways to
help k0s and the k0sproject community as a whole:

- **Report bugs** via [GitHub issues].  
  A clear report with steps to reproduce, the k0s version, and relevant logs is
  a valuable contribution on its own.
- **Help others** in [GitHub issues] and on [#k0s-users].  
  Answering a question, sharing how you solved a problem, or adding what you
  know to an issue someone else reported helps the community.
- **Help the maintainers** by testing and reviewing [pull requests].  
  Trying out a change in your environment, or reading it with a fresh pair of
  eyes, is valuable feedback that speeds up the review.
- **Share your experience.**  
  Write a [blog post][k0s-blog], give a talk, or tell us about your use case.
  Add yourself to the list of [adopters], and let us know on Slack, so we can
  spread the word.
- **Join the [community calls](../README.md#community-calls).**  
  Discuss ideas, ask questions, or just get to know the people behind k0s.

[GitHub issues]: https://github.com/k0sproject/k0s/issues
[#k0s-users]: https://kubernetes.slack.com/archives/C0809EA06QZ
[pull requests]: https://github.com/k0sproject/k0s/pulls
[k0s-blog]: https://github.com/k0sproject/blog
[adopters]: https://github.com/k0sproject/k0s/blob/main/ADOPTERS.md

## License

By contributing, you agree that your contributions will be licensed as follows:

- All content residing under the "docs/" directory of this repository is licensed under "Creative Commons Attribution Share Alike 4.0 International" (CC-BY-SA-4.0). See docs/LICENSE for details.
- Content outside of the above mentioned directories or restrictions above is available under the "Apache License 2.0".
