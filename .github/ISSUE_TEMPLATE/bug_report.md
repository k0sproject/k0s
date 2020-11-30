---
name: Bug report
about: Create a report to help us improve
title: ''
labels: bug
assignees: ''

---

<!--
Please use this template while reporting a bug and provide as much info as possible. Not doing so may result in your bug not being addressed in a timely manner. Thanks!

Before creating an issue, make sure you've checked the following:
- You are running the latest released version of k0s
- Make sure you've searched for existing issues, both open and closed
- Make sure you've searched for PRs too, a fix might've been merged already
- You're looking at docs for the released version, `main` branch docs are usually ahead of released versions.
    - Docs for exact released version can be found at https://github.com/k0sproject/k0s/tree/<version>/docs

-->

**Version**
```
$ k0s version
```
**Platform**
Which platform did you run k0s on?
```
$ lsb_release -a
```
**What happened?**
A clear and concise description of what the bug is.

**How To Reproduce**
How can we reproduce this issue? (as minimally and as precisely as possible)

**Expected behavior**
A clear and concise description of what you expected to happen.

**Screenshots & Logs**
If applicable, add screenshots to help explain your problem.
Also add any output from kubectl if applicable:
```
$ export KUBECONFIG=/var/lib/k0s/pki/admin.conf
$ kubectl logs ...
```

**Additional context**
Add any other context about the problem here.
