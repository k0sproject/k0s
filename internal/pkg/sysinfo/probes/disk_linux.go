/*
Copyright 2022 k0s authors

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

package probes

import (
	"fmt"
	"os"
	"path"

	"golang.org/x/sys/unix"
)

func (a *assertDiskSpace) Probe(reporter Reporter) error {
	var stat unix.Statfs_t
	for p := a.fsPath; ; {
		if err := unix.Statfs(p, &stat); err != nil {
			if os.IsNotExist(err) {
				if parent := path.Dir(p); parent != p {
					p = parent
					continue
				}
			}
			return reporter.Error(a.desc(), err)
		}

		break
	}

	// https://stackoverflow.com/a/20110856
	// Available blocks * size per block = available space in bytes
	free := stat.Bavail * uint64(stat.Bsize)
	if a.isRelative {
		percentFree := 100 * free / (stat.Blocks * uint64(stat.Bsize))
		return a.reportPercent(reporter, percentFree)
	}
	return a.reportBytes(reporter, free)
}

func (a *assertDiskSpace) reportPercent(reporter Reporter, percentFree uint64) error {
	if percentFree >= a.minFree {
		return reporter.Pass(a.desc(), StringProp(fmt.Sprintf("%d%%", percentFree)))
	}
	return reporter.Warn(a.desc(), StringProp(fmt.Sprintf("%d%%", percentFree)), fmt.Sprintf("%d%% recommended", a.minFree))
}

func (a *assertDiskSpace) reportBytes(reporter Reporter, bytesFree uint64) error {
	if bytesFree >= a.minFree {
		return reporter.Pass(a.desc(), iecBytes(bytesFree))
	}
	return reporter.Warn(a.desc(), iecBytes(bytesFree), fmt.Sprintf("%s recommended", iecBytes(a.minFree)))
}
