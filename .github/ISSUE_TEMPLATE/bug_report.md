---
name: Bug report
about: Create a report to help us improve
title: ''
labels: ''
assignees: ''

---

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
