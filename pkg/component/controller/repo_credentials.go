// SPDX-FileCopyrightText: 2026 k0s authors
// SPDX-License-Identifier: Apache-2.0

package controller

import (
	"context"
	"fmt"

	k0sv1beta1 "github.com/k0sproject/k0s/pkg/apis/k0s/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// resolveBasicAuthSecret fetches and extracts credentials from a Kubernetes secret
// Returns (username, password, isPermanentError, error)
func resolveBasicAuthSecret(ctx context.Context, client kubernetes.Interface, secretRef k0sv1beta1.SecretReference, repoName string) (string, string, bool, error) {
	secret, err := client.CoreV1().Secrets(secretRef.Namespace).Get(ctx, secretRef.Name, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			// Missing secret is retriable - it might be created later
			return "", "", false, fmt.Errorf("secret %s/%s not found for repository %q", secretRef.Namespace, secretRef.Name, repoName)
		}
		// Other errors (permission, API server down, etc.) are retriable
		return "", "", false, fmt.Errorf("failed to fetch secret %s/%s for repository %q: %w", secretRef.Namespace, secretRef.Name, repoName, err)
	}

	// Verify secret type
	if secret.Type != corev1.SecretTypeBasicAuth {
		// Wrong secret type is permanent - user needs to fix the configuration
		return "", "", true, fmt.Errorf("secret %s/%s for repository %q has type %q, expected %q",
			secretRef.Namespace, secretRef.Name, repoName, secret.Type, corev1.SecretTypeBasicAuth)
	}

	// Extract credentials
	username, hasUsername := secret.Data[corev1.BasicAuthUsernameKey]
	password, hasPassword := secret.Data[corev1.BasicAuthPasswordKey]

	if !hasUsername || !hasPassword {
		// Missing keys is permanent - user needs to fix the secret
		return "", "", true, fmt.Errorf("secret %s/%s for repository %q is missing required keys %q and/or %q",
			secretRef.Namespace, secretRef.Name, repoName, corev1.BasicAuthUsernameKey, corev1.BasicAuthPasswordKey)
	}

	return string(username), string(password), false, nil
}
