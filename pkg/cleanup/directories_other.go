//go:build !linux

/*
Copyright 2021 k0s authors

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

package cleanup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"
	"k8s.io/mount-utils"
)

type directories struct {
	Config *Config
}

// Name returns the name of the step
func (d *directories) Name() string {
	return "remove directories step"
}

// Run removes all kubelet mounts and deletes generated dataDir and runDir
func (d *directories) Run() error {
	// unmount any leftover overlays (such as in alpine)
	mounter := mount.New("")
	procMounts, err := mounter.List()
	if err != nil {
		return err
	}

	var dataDirMounted bool

	// search and unmount kubelet volume mounts
	for _, v := range procMounts {
		if v.Path == filepath.Join(d.Config.dataDir, "kubelet") {
			logrus.Debugf("%v is mounted! attempting to unmount...", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				logrus.Warningf("failed to unmount %v", v.Path)
			}
		} else if v.Path == d.Config.dataDir {
			dataDirMounted = true
		}
	}

	if dataDirMounted {
		logrus.Debugf("removing the contents of mounted data-dir (%s)", d.Config.dataDir)
	} else {
		logrus.Debugf("removing k0s generated data-dir (%s)", d.Config.dataDir)
	}

	if err := os.RemoveAll(d.Config.dataDir); err != nil {
		if !dataDirMounted {
			return fmt.Errorf("failed to delete k0s generated data-dir: %w", err)
		}
		if !errorIsUnlinkat(err, d.Config.dataDir) {
			return fmt.Errorf("failed to delete contents of mounted data-dir: %w", err)
		}
	}

	logrus.Debugf("deleting k0s generated run-dir (%s)", d.Config.runDir)
	if err := os.RemoveAll(d.Config.runDir); err != nil {
		return fmt.Errorf("failed to delete %s: %w", d.Config.runDir, err)
	}

	return nil
}

// this is for checking if the error returned by os.RemoveAll is due to
// it being a mount point. if it is, we can ignore the error. this way
// we can't rely on os.RemoveAll instead of recursively deleting the
// contents of the directory
func errorIsUnlinkat(err error, dir string) bool {
	if err == nil {
		return false
	}
	var pathErr *os.PathError
	if !errors.As(err, &pathErr) {
		return false
	}
	return pathErr.Path == dir && pathErr.Op == "unlinkat"
}
