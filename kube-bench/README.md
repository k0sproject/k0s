# KUBE-BENCH

- CIS compliance verification tool

## Install

Follow these instructions https://github.com/aquasecurity/kube-bench/#installation

## Run

After successfully installing the kube-bench make sure you have running k0s single node cluster. Then execute following command:

```
$ kube-bench run  --config-dir kube-bench/cfg/ --benchmark k0s-1.0
```

## Summary

Current configuration has in total of 13 master checks disabled: 

```
id: 1.2.1
        type: skip
        text: "Ensure that the --anonymous-auth argument is set to false (Manual)"
id: 1.2.10
        type: skip
        text: "Ensure that the admission control plugin EventRateLimit is set (Manual)"
id: 1.2.12
        type: skip
        text: "Ensure that the admission control plugin AlwaysPullImages is set (Manual)"
id: 1.2.13
        type: skip
        text: "Ensure that the admission control plugin SecurityContextDeny is set if PodSecurityPolicy is not used (Manual)"
id: 1.2.16
        type: skip
        text: "Ensure that the admission control plugin PodSecurityPolicy is set (Automated)"
id: 1.2.22
        type: skip
        text: "Ensure that the --audit-log-path argument is set (Automated)"
id: 1.2.23
        type: skip
        text: "Ensure that the --audit-log-maxage argument is set to 30 or as appropriate (Automated)"
id: 1.2.24
        type: skip
        text: "Ensure that the --audit-log-maxbackup argument is set to 10 or as appropriate (Automated)"
id: 1.2.25
        type: skip
        text: "Ensure that the --audit-log-maxsize argument is set to 100 or as appropriate (Automated)"
id: 1.2.33
        type: skip
        text: "Ensure that the --encryption-provider-config argument is set as appropriate (Manual)"
id: 1.2.34
        type: skip
        text: "Ensure that encryption providers are appropriately configured (Manual)"
id: 1.2.35
        type: skip
        text: "Ensure that the API Server only makes use of Strong Cryptographic Ciphers (Manual)"
id: 1.3.1
        type: skip
        text: "Ensure that the --terminated-pod-gc-threshold argument is set as appropriate (Manual)"
```

and 5 node checks disabled:
```
id: 4.1.1
        type: skip
        text: "Ensure that the kubelet service file permissions are set to 644 or more restrictive (Automated)"
id: 4.1.2
        type: skip
        text: "Ensure that the kubelet service file ownership is set to root:root (Automated)"
id: 4.2.6
        type: skip
        text: "Ensure that the --protect-kernel-defaults argument is set to true (Automated)"
id: 4.2.9
        type: skip
        text: "Ensure that the --event-qps argument is set to 0 or a level which ensures appropriate event capture (Manual)"
id: 4.2.10
        type: skip
        text: "Ensure that the --tls-cert-file and --tls-private-key-file arguments are set as appropriate (Manual)"
```