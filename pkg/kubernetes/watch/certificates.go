// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package watch

import (
	certificatesv1 "k8s.io/api/certificates/v1"
)

func CertificateSigningRequests(client Provider[*certificatesv1.CertificateSigningRequestList]) *Watcher[certificatesv1.CertificateSigningRequest] {
	return FromClient[*certificatesv1.CertificateSigningRequestList, certificatesv1.CertificateSigningRequest](client)
}
