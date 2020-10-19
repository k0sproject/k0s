[INFO] 1 Master Node Security Configuration
[INFO] 1.1 Master Node Configuration Files
[PASS] 1.1.1 Ensure that the API server pod specification file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.2 Ensure that the API server pod specification file ownership is set to root:root (Automated)
[INFO] 1.1.3 Ensure that the controller manager pod specification file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.4 Ensure that the controller manager pod specification file ownership is set to root:root (Automated)
[INFO] 1.1.5 Ensure that the scheduler pod specification file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.6 Ensure that the scheduler pod specification file ownership is set to root:root (Automated)
[INFO] 1.1.7 Ensure that the etcd pod specification file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.8 Ensure that the etcd pod specification file ownership is set to root:root (Automated)
[INFO] 1.1.9 Ensure that the Container Network Interface file permissions are set to 644 or more restrictive (Manual)
[INFO] 1.1.10 Ensure that the Container Network Interface file ownership is set to root:root (Manual)
[INFO] 1.1.11 Ensure that the etcd data directory permissions are set to 700 or more restrictive (Automated)
[PASS] 1.1.12 Ensure that the etcd data directory ownership is set to etcd:etcd (Automated)
[INFO] 1.1.13 Ensure that the admin.conf file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.14 Ensure that the admin.conf file ownership is set to root:root (Automated)
[INFO] 1.1.15 Ensure that the scheduler.conf file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.16 Ensure that the scheduler.conf file ownership is set to root:root (Automated)
[INFO] 1.1.17 Ensure that the controller-manager.conf file permissions are set to 644 or more restrictive (Automated)
[INFO] 1.1.18 Ensure that the controller-manager.conf file ownership is set to root:root (Automated)
[INFO] 1.1.19 Ensure that the Kubernetes PKI directory and file ownership is set to root:root (Automated)
[INFO] 1.1.20 Ensure that the Kubernetes PKI certificate file permissions are set to 644 or more restrictive (Manual)
[INFO] 1.1.21 Ensure that the Kubernetes PKI key file permissions are set to 600 (Manual)
[INFO] 1.2 API Server
[WARN] 1.2.1 Ensure that the --anonymous-auth argument is set to false (Manual)
[PASS] 1.2.2 Ensure that the --basic-auth-file argument is not set (Automated)
[PASS] 1.2.3 Ensure that the --token-auth-file parameter is not set (Automated)
[PASS] 1.2.4 Ensure that the --kubelet-https argument is set to true (Automated)
[FAIL] 1.2.5 Ensure that the --kubelet-client-certificate and --kubelet-client-key arguments are set as appropriate (Automated)
[PASS] 1.2.6 Ensure that the --kubelet-certificate-authority argument is set as appropriate (Automated)
[FAIL] 1.2.7 Ensure that the --authorization-mode argument is not set to AlwaysAllow (Automated)
[FAIL] 1.2.8 Ensure that the --authorization-mode argument includes Node (Automated)
[FAIL] 1.2.9 Ensure that the --authorization-mode argument includes RBAC (Automated)
[WARN] 1.2.10 Ensure that the admission control plugin EventRateLimit is set (Manual)
[FAIL] 1.2.11 Ensure that the admission control plugin AlwaysAdmit is not set (Automated)
[WARN] 1.2.12 Ensure that the admission control plugin AlwaysPullImages is set (Manual)
[WARN] 1.2.13 Ensure that the admission control plugin SecurityContextDeny is set if PodSecurityPolicy is not used (Manual)
[PASS] 1.2.14 Ensure that the admission control plugin ServiceAccount is set (Automated)
[PASS] 1.2.15 Ensure that the admission control plugin NamespaceLifecycle is set (Automated)
[FAIL] 1.2.16 Ensure that the admission control plugin PodSecurityPolicy is set (Automated)
[FAIL] 1.2.17 Ensure that the admission control plugin NodeRestriction is set (Automated)
[PASS] 1.2.18 Ensure that the --insecure-bind-address argument is not set (Automated)
[FAIL] 1.2.19 Ensure that the --insecure-port argument is set to 0 (Automated)
[FAIL] 1.2.20 Ensure that the --secure-port argument is not set to 0 (Automated)
[PASS] 1.2.21 Ensure that the --profiling argument is set to false (Automated)
[PASS] 1.2.22 Ensure that the --audit-log-path argument is set (Automated)
[PASS] 1.2.23 Ensure that the --audit-log-maxage argument is set to 30 or as appropriate (Automated)
[PASS] 1.2.24 Ensure that the --audit-log-maxbackup argument is set to 10 or as appropriate (Automated)
[PASS] 1.2.25 Ensure that the --audit-log-maxsize argument is set to 100 or as appropriate (Automated)
[PASS] 1.2.26 Ensure that the --request-timeout argument is set as appropriate (Automated)
[PASS] 1.2.27 Ensure that the --service-account-lookup argument is set to true (Automated)
[FAIL] 1.2.28 Ensure that the --service-account-key-file argument is set as appropriate (Automated)
[FAIL] 1.2.29 Ensure that the --etcd-certfile and --etcd-keyfile arguments are set as appropriate (Automated)
[FAIL] 1.2.30 Ensure that the --tls-cert-file and --tls-private-key-file arguments are set as appropriate (Automated)
[FAIL] 1.2.31 Ensure that the --client-ca-file argument is set as appropriate (Automated)
[FAIL] 1.2.32 Ensure that the --etcd-cafile argument is set as appropriate (Automated)
[PASS] 1.2.33 Ensure that the --encryption-provider-config argument is set as appropriate (Manual)
[WARN] 1.2.34 Ensure that encryption providers are appropriately configured (Manual)
[PASS] 1.2.35 Ensure that the API Server only makes use of Strong Cryptographic Ciphers (Manual)
[INFO] 1.3 Controller Manager
[PASS] 1.3.1 Ensure that the --terminated-pod-gc-threshold argument is set as appropriate (Manual)
[PASS] 1.3.2 Ensure that the --profiling argument is set to false (Automated)
[FAIL] 1.3.3 Ensure that the --use-service-account-credentials argument is set to true (Automated)
[FAIL] 1.3.4 Ensure that the --service-account-private-key-file argument is set as appropriate (Automated)
[FAIL] 1.3.5 Ensure that the --root-ca-file argument is set as appropriate (Automated)
[PASS] 1.3.6 Ensure that the RotateKubeletServerCertificate argument is set to true (Automated)
[FAIL] 1.3.7 Ensure that the --bind-address argument is set to 127.0.0.1 (Automated)
[INFO] 1.4 Scheduler
[PASS] 1.4.1 Ensure that the --profiling argument is set to false (Automated)
[FAIL] 1.4.2 Ensure that the --bind-address argument is set to 127.0.0.1 (Automated)

== Remediations ==
1.2.1 Edit the API server pod specification file apiserver
on the master node and set the below parameter.
--anonymous-auth=false

1.2.5 Follow the Kubernetes documentation and set up the TLS connection between the
apiserver and kubelets. Then, edit API server pod specification file
apiserver on the master node and set the
kubelet client certificate and key parameters as below.
--kubelet-client-certificate=<path/to/client-certificate-file>
--kubelet-client-key=<path/to/client-key-file>

1.2.7 Edit the API server pod specification file apiserver
on the master node and set the --authorization-mode parameter to values other than AlwaysAllow.
One such example could be as below.
--authorization-mode=RBAC

1.2.8 Edit the API server pod specification file apiserver
on the master node and set the --authorization-mode parameter to a value that includes Node.
--authorization-mode=Node,RBAC

1.2.9 Edit the API server pod specification file apiserver
on the master node and set the --authorization-mode parameter to a value that includes RBAC,
for example:
--authorization-mode=Node,RBAC

1.2.10 Follow the Kubernetes documentation and set the desired limits in a configuration file.
Then, edit the API server pod specification file apiserver
and set the below parameters.
--enable-admission-plugins=...,EventRateLimit,...
--admission-control-config-file=<path/to/configuration/file>

1.2.11 Edit the API server pod specification file apiserver
on the master node and either remove the --enable-admission-plugins parameter, or set it to a
value that does not include AlwaysAdmit.

1.2.12 Edit the API server pod specification file apiserver
on the master node and set the --enable-admission-plugins parameter to include
AlwaysPullImages.
--enable-admission-plugins=...,AlwaysPullImages,...

1.2.13 Edit the API server pod specification file apiserver
on the master node and set the --enable-admission-plugins parameter to include
SecurityContextDeny, unless PodSecurityPolicy is already in place.
--enable-admission-plugins=...,SecurityContextDeny,...

1.2.16 Follow the documentation and create Pod Security Policy objects as per your environment.
Then, edit the API server pod specification file apiserver
on the master node and set the --enable-admission-plugins parameter to a
value that includes PodSecurityPolicy:
--enable-admission-plugins=...,PodSecurityPolicy,...
Then restart the API Server.

1.2.17 Follow the Kubernetes documentation and configure NodeRestriction plug-in on kubelets.
Then, edit the API server pod specification file apiserver
on the master node and set the --enable-admission-plugins parameter to a
value that includes NodeRestriction.
--enable-admission-plugins=...,NodeRestriction,...

1.2.19 Edit the API server pod specification file apiserver
on the master node and set the below parameter.
--insecure-port=0

1.2.20 Edit the API server pod specification file apiserver
on the master node and either remove the --secure-port parameter or
set it to a different (non-zero) desired port.

1.2.28 Edit the API server pod specification file apiserver
on the master node and set the --service-account-key-file parameter
to the public key file for service accounts:
--service-account-key-file=<filename>

1.2.29 Follow the Kubernetes documentation and set up the TLS connection between the apiserver and etcd.
Then, edit the API server pod specification file apiserver
on the master node and set the etcd certificate and key file parameters.
--etcd-certfile=<path/to/client-certificate-file>
--etcd-keyfile=<path/to/client-key-file>

1.2.30 Follow the Kubernetes documentation and set up the TLS connection on the apiserver.
Then, edit the API server pod specification file apiserver
on the master node and set the TLS certificate and private key file parameters.
--tls-cert-file=<path/to/tls-certificate-file>
--tls-private-key-file=<path/to/tls-key-file>

1.2.31 Follow the Kubernetes documentation and set up the TLS connection on the apiserver.
Then, edit the API server pod specification file apiserver
on the master node and set the client certificate authority file.
--client-ca-file=<path/to/client-ca-file>

1.2.32 Follow the Kubernetes documentation and set up the TLS connection between the apiserver and etcd.
Then, edit the API server pod specification file apiserver
on the master node and set the etcd certificate authority file parameter.
--etcd-cafile=<path/to/ca-file>

1.2.34 Follow the Kubernetes documentation and configure a EncryptionConfig file.
In this file, choose aescbc, kms or secretbox as the encryption provider.

1.3.3 Edit the Controller Manager pod specification file controllermanager
on the master node to set the below parameter.
--use-service-account-credentials=true

1.3.4 Edit the Controller Manager pod specification file controllermanager
on the master node and set the --service-account-private-key-file parameter
to the private key file for service accounts.
--service-account-private-key-file=<filename>

1.3.5 Edit the Controller Manager pod specification file controllermanager
on the master node and set the --root-ca-file parameter to the certificate bundle file`.
--root-ca-file=<path/to/file>

1.3.7 Edit the Controller Manager pod specification file controllermanager
on the master node and ensure the correct value for the --bind-address parameter

1.4.2 Edit the Scheduler pod specification file scheduler
on the master node and ensure the correct value for the --bind-address parameter


== Summary ==
22 checks PASS
19 checks FAIL
5 checks WARN
19 checks INFO
