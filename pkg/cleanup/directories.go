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
	"fmt"
	"os"
	"strings"

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

	// search and unmount kubelet volume mounts
	for _, v := range procMounts {
		if strings.Compare(v.Path, fmt.Sprintf("%s/kubelet", d.Config.dataDir)) == 0 {
			logrus.Debugf("%v is mounted! attempting to unmount...", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				logrus.Warningf("failed to unmount %v", v.Path)
			}
		} else if strings.Compare(v.Path, d.Config.dataDir) == 0 {
			logrus.Debugf("%v is mounted! attempting to unmount...", v.Path)
			if err = mounter.Unmount(v.Path); err != nil {
				logrus.Warningf("failed to unmount %v", v.Path)
			}
		}
	}

	logrus.Debugf("deleting k0s generated data-dir (%v) and run-dir (%v)", d.Config.dataDir, d.Config.runDir)
	if err := os.RemoveAll(d.Config.dataDir); err != nil {
		fmtError := fmt.Errorf("failed to delete %v. err: %v", d.Config.dataDir, err)
		return fmtError
	}
	if err := os.RemoveAll(d.Config.runDir); err != nil {
		fmtError := fmt.Errorf("failed to delete %v. err: %v", d.Config.runDir, err)
		return fmtError
	}

	return nil
}
