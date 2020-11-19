package constant

// WinCertCA defines the CA.crt location.
// this one is defined here because it is used not only on windows worker but also during the control plane bootstrap
const WinCertCA = "C:\\var\\lib\\k0s\\pki\\ca.crt"
