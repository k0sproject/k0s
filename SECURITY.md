# Security policy

This policy is adapted from the policies of various other mature CNCF projects:

* [Crossplane]
* [Rook]
* [Containerd]

[Crossplane]: https://github.com/crossplane/crossplane/blob/main/SECURITY.md
[Rook]: https://github.com/rook/rook
[Containerd]: https://github.com/containerd/containerd

## Supported versions

We follow the upstream Kubernetes [EOL policy] which means the following versions are supported and maintained:

| Version   | Supported |
|-----------|-----------|
| v1.34.x   | ✅        |
| v1.33.x   | ✅        |
| v1.32.x   | ✅        |
| < v1.32.x | ❌        |

[EOL policy]: https://kubernetes.io/releases/patch-releases/

## Reporting a vulnerability

k0s supports responsible disclosure and endeavors to resolve security issues in a reasonable timeframe.

To report a vulnerability, either:

1. Report it on Github directly by following the procedure described here:

   * Navigate to the [Security tab] on the repository
   * Click on 'Advisories'
   * Click on 'Report a vulnerability'
   * Detail the issue, see below for some expamples of info that might be useful including.

2. Send an email to cncf-k0s-maintainers@lists.cncf.io detailing the issue, see below for some examples of info that might be useful including.

    The reporter(s) can typically expect a response within 24 hours acknowledging the issue was received. If a response is not received within 24 hours, please reach out to any of the [maintainers] directly to confirm receipt of the issue.

Prefer using GitHub's [private security reporting] system as it provides a secure channel for communication and allows the reporter and maintainers to coordinate the disclosure and the fix before public disclosure.

[private security reporting]: https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing-information-about-vulnerabilities/privately-reporting-a-security-vulnerability
[Security tab]: https://github.com/k0sproject/k0s/security
[maintainers]: MAINTAINERS.md

### Report Content

Make sure to include all the details that might help maintainers better
understand and prioritize it, for example here is a list of details that might be
worth adding:

* Versions of k0s used and more broadly of any other software involved (OS, container runtime, CNI plugin, etc).
* Configuration used (k0s.yaml, command line flags, etc).
* Detailed list of steps to reproduce the vulnerability.
* Consequences of the vulnerability.
* Severity you feel should be attributed to the vulnerabilities.
* Screenshots, logs or Kubernetes Events

Feel free to extend the list above with everything else you think would be
useful.

## Review Process

Once a maintainer has confirmed the relevance of the report, a draft security
advisory will be created on Github. The draft advisory will be used to discuss
the issue with maintainers and the reporter(s). If the reporter(s) wishes to participate in this discussion, then provide
reporter Github username(s) to be invited to the discussion. If the reporter(s)
does not wish to participate directly in the discussion, then the reporter(s)
can request to be updated regularly via email.

If the vulnerability is accepted, a timeline for developing a patch, public
disclosure, and patch release will be determined in coordination with the maintainers and reporter(s).
The reporter(s) are expectedto participate in the discussion of the timeline and abide by agreed upon dates
for public disclosure.

## Public Disclosure Process

Vulnerabilities once fixed, will be shared publicly as a Github [security advisory] and mentioned in the fixed versions' release notes. Maintainers will also endeavor to mention the fixed vulnerabilities in the k0s project's blog and notify the general community via the Slack and other communication channels.

[security advisory]: https://docs.github.com/en/code-security/security-advisories/about-security-advisories