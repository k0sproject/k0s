// SPDX-FileCopyrightText: 2020 k0s authors
// SPDX-License-Identifier: Apache-2.0

package applier

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/k0sproject/k0s/pkg/kubernetes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/sirupsen/logrus"
)

// manifestFilePattern is the glob pattern that all applicable manifest files need to match.
const manifestFilePattern = "*.yaml"

func FindManifestFilesInDir(dir string) ([]string, error) {
	// Use a map to avoid duplicates
	fileMap := make(map[string]bool)
	
	// First, try to resolve any symlinks in the directory path
	resolvedDir, err := filepath.EvalSymlinks(dir)
	if err != nil {
		// If we can't resolve the directory path, use the original directory
		// This can happen if the directory contains broken symlinks
		resolvedDir = dir
	}
	
	// Get all yaml files in the resolved directory
	files, err := filepath.Glob(filepath.Join(resolvedDir, manifestFilePattern))
	if err != nil {
		return nil, err
	}
	
	for _, file := range files {
		// Check if the file is a symlink and if it's broken
		info, err := os.Lstat(file)
		if err != nil {
			continue // Skip if we can't even stat the file
		}
		
		if info.Mode()&os.ModeSymlink != 0 {
			// It's a symlink, check if it's broken
			_, err := os.Stat(file)
			if err != nil {
				continue // Skip broken symlinks
			}
		}
		
		fileMap[file] = true
	}
	
	// Also check for symlinks in the original directory that might point to yaml files
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	
	for _, entry := range entries {
		fullPath := filepath.Join(dir, entry.Name())
		
		// Check if it's a symlink
		info, err := os.Lstat(fullPath)
		if err != nil {
			continue
		}
		
		if info.Mode()&os.ModeSymlink != 0 {
			// Resolve the symlink
			resolvedPath, err := filepath.EvalSymlinks(fullPath)
			if err != nil {
				// Skip broken symlinks
				continue
			}
			
			// Check if the resolved path exists and matches our pattern
			if match, _ := filepath.Match(manifestFilePattern, filepath.Base(resolvedPath)); match {
				// Verify the resolved file actually exists
				if _, err := os.Stat(resolvedPath); err == nil {
					// Add the original symlink path, not the resolved path
					// This ensures we track changes to the symlink itself
					fileMap[fullPath] = true
				}
			}
		}
	}
	
	// Convert map back to slice
	result := make([]string, 0, len(fileMap))
	for file := range fileMap {
		result = append(result, file)
	}
	
	return result, nil
}

// Applier manages all the "static" manifests and applies them on the k8s API
type Applier struct {
	Name string
	Dir  string

	log           *logrus.Entry
	clientFactory kubernetes.ClientFactoryInterface
}

// NewApplier creates new Applier
func NewApplier(dir string, kubeClientFactory kubernetes.ClientFactoryInterface) Applier {
	name := filepath.Base(dir)
	log := logrus.WithFields(logrus.Fields{
		"component": "applier",
		"bundle":    name,
	})

	return Applier{
		log:           log,
		Dir:           dir,
		Name:          name,
		clientFactory: kubeClientFactory,
	}
}

// Apply resources
func (a *Applier) Apply(ctx context.Context) error {
	files, err := FindManifestFilesInDir(a.Dir)
	if err != nil {
		return err
	}

	resources, err := a.parseFiles(files)
	if err != nil {
		return err
	}
	stack := Stack{
		Name:      a.Name,
		Resources: resources,
		Clients:   a.clientFactory,
	}
	a.log.Debug("applying stack")
	err = stack.Apply(ctx, true)
	if err != nil {
		a.log.WithError(err).Warn("stack apply failed")
	} else {
		a.log.Debug("successfully applied stack")
	}

	return err
}

// Delete deletes the entire stack by applying it with empty set of resources
func (a *Applier) Delete(ctx context.Context) error {
	stack := Stack{Name: a.Name, Clients: a.clientFactory}
	logrus.Debugf("about to delete a stack %s with empty apply", a.Name)
	return stack.Apply(ctx, true)
}

func (a *Applier) parseFiles(files []string) ([]*unstructured.Unstructured, error) {
	var resources []*unstructured.Unstructured
	if len(files) == 0 {
		return resources, nil
	}

	objects, err := resource.NewLocalBuilder().
		Unstructured().
		Path(false, files...).
		Flatten().
		Do().
		Infos()
	if err != nil {
		return nil, fmt.Errorf("unable to build resources: %w", err)
	}
	for _, o := range objects {
		item := o.Object.(*unstructured.Unstructured)
		if item.GetAPIVersion() != "" && item.GetKind() != "" {
			resources = append(resources, item)
		}
	}

	return resources, nil
}
