/*
Copyright 2024 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package oras

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"oras.land/oras-go/v2"
	"oras.land/oras-go/v2/content/file"
	"oras.land/oras-go/v2/registry"
	"oras.land/oras-go/v2/registry/remote"
	"oras.land/oras-go/v2/registry/remote/auth"
	"oras.land/oras-go/v2/registry/remote/credentials"
)

// Download downloads the oras artifact present at the given registry URL.
// Usage example:
//
// artifact := "docker.io/company/k0s:latest"
// fp, _ := os.CreateTemp("", "k0s-oras-artifact-*")
// err := oras.Download(ctx, artifact, fp)
//
// This function expects only one artifact to be present, if none is found this
// returns an error. The artifact is downloaded in a temporary location before
// being copied to the target.
func Download(ctx context.Context, url string, target io.Writer, options ...OrasOption) (err error) {
	opts := orasOptions{}
	for _, opt := range options {
		opt(&opts)
	}

	creds, err := opts.auth.CredentialStore(ctx)
	if err != nil {
		return fmt.Errorf("failed to create credential store: %w", err)
	}

	imgref, err := registry.ParseReference(url)
	if err != nil {
		return fmt.Errorf("failed to parse artifact reference: %w", err)
	}

	tmpdir, err := os.MkdirTemp("", "k0s-oras-artifact-*")
	if err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	defer func() { _ = os.RemoveAll(tmpdir) }()

	repo, err := remote.NewRepository(url)
	if err != nil {
		return fmt.Errorf("failed to create repository: %w", err)
	}

	fs, err := file.New(tmpdir)
	if err != nil {
		return fmt.Errorf("failed to create file store: %w", err)
	}
	defer fs.Close()

	transp := http.DefaultTransport.(*http.Transport).Clone()
	if opts.insecureSkipTLSVerify {
		transp.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	repo.Client = &auth.Client{
		Client:     &http.Client{Transport: transp},
		Credential: creds.Get,
	}

	tag := imgref.Reference
	if _, err := oras.Copy(ctx, repo, tag, fs, tag, oras.DefaultCopyOptions); err != nil {
		return fmt.Errorf("failed to fetch artifact: %w", err)
	}

	files, err := os.ReadDir(tmpdir)
	if err != nil {
		return fmt.Errorf("failed to read temp dir: %w", err)
	}

	// we always expect only one single file to be downloaded.
	if len(files) == 0 {
		return fmt.Errorf("no artifacts found")
	} else if len(files) > 1 {
		return fmt.Errorf("multiple artifacts found")
	}

	fpath := filepath.Join(tmpdir, files[0].Name())
	fp, err := os.Open(fpath)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer fp.Close()

	if _, err := io.Copy(target, fp); err != nil {
		return fmt.Errorf("failed to copy file: %w", err)
	}

	return nil
}

// orasOptions holds the options used when downloading oras artifacts.
type orasOptions struct {
	insecureSkipTLSVerify bool
	auth                  DockerConfig
}

// OrasOptions is a function that sets an option for the oras download.
type OrasOption func(*orasOptions)

// WithInsecureSkipTLSVerify sets the insecureSkipTLSVerify option to true.
func WithInsecureSkipTLSVerify() OrasOption {
	return func(opts *orasOptions) {
		opts.insecureSkipTLSVerify = true
	}
}

// WithDockerAuth sets the Docker config to be used when authenticating to
// the registry.
func WithDockerAuth(auth DockerConfig) OrasOption {
	return func(opts *orasOptions) {
		opts.auth = auth
	}
}

// DockerConfigEntry holds an entry in the '.dockerconfigjson' file.
type DockerConfigEntry struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// DockerConfig represents the content of the '.dockerconfigjson' file.
type DockerConfig struct {
	Auths map[string]DockerConfigEntry `json:"auths"`
}

// CredentialStore turns the Docker configuration into a credential store and
// returns it.
func (d DockerConfig) CredentialStore(ctx context.Context) (credentials.Store, error) {
	creds := credentials.NewMemoryStore()
	for addr, entry := range d.Auths {
		if err := creds.Put(ctx, addr, auth.Credential{
			Username: entry.Username,
			Password: entry.Password,
		}); err != nil {
			return nil, fmt.Errorf("failed to add credential: %w", err)
		}
	}
	return creds, nil
}
