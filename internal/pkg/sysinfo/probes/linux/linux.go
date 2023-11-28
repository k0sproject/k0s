//go:build linux

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

package linux

import (
	"fmt"
	"sync"
	"syscall"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

// revive:disable-next-line:exported
type LinuxProbes struct {
	probes.Probes

	path         probes.ProbePath
	probeUname   unameProber
	probeKConfig kConfigProber
}

func RequireLinux(parent probes.ParentProbe) (l *LinuxProbes) {
	parent.Set("os", func(path probes.ProbePath, current probes.Probe) probes.Probe {
		var ok bool
		if l, ok = current.(*LinuxProbes); ok {
			return l
		}

		l = newLinuxProbes(path)
		return l
	})

	return
}

func newLinuxProbes(path probes.ProbePath) *LinuxProbes {
	unameProber := newUnameProber()
	return &LinuxProbes{
		probes.NewProbesAtPath(path),

		path,
		unameProber,
		newKConfigProber(unameProber),
	}
}

func (l *LinuxProbes) Probe(reporter probes.Reporter) error {
	if err := l.probe(reporter); err != nil {
		return err
	}

	return l.Probes.Probe(reporter)
}

func (l *LinuxProbes) probe(reporter probes.Reporter) error {
	desc := probes.NewProbeDesc("Operating system", l.path)
	//revive:disable:indent-error-flow
	if uname, err := l.probeUname(); err != nil {
		return reporter.Error(desc, err)
	} else if uname.osName.value == "Linux" {
		return reporter.Pass(desc, uname.osName)
	} else {
		return reporter.Reject(desc, uname.osName, "Linux required")
	}
}

// unameField represents a field as returned from the uname syscall.
type unameField struct {
	// value is the value returned, converted to a string
	value string
	// truncated indicates if the value is potentially truncated, i.e. the
	// buffer for the return value was full.
	truncated bool
}

func (f unameField) String() string {
	if f.truncated {
		return fmt.Sprintf("%q (truncated)", f.value)
	}
	return f.value

}

// uname represents data as returned by the uname syscall.
type uname struct {
	// osName represents the operating system name (e.g., "Linux")
	osName unameField
	// nodeName represents a name within "some implementation-defined network"
	nodeName unameField
	// osRelease represents the operating system release (e.g., "2.6.28")
	osRelease unameField
	// osVersion represents the operating system version
	osVersion unameField
	// arch represents some hardware identifier (e.g., "x86_64")
	arch unameField
}

type unameProber func() (*uname, error)

func newUnameProber() unameProber {
	var once sync.Once
	var loaded *uname
	var err error

	return func() (*uname, error) {
		once.Do(func() {
			var utsname syscall.Utsname
			if err = syscall.Uname(&utsname); err != nil {
				err = fmt.Errorf("uname syscall failed: %w", err)
			} else {
				loaded = parseUname(&utsname)
			}
		})

		return loaded, err
	}
}

func parseUname(utsname *syscall.Utsname) *uname {
	convert := func(chars utsStringPtr) unameField {
		var buf [65]byte
		var i int
		for pos, ch := range *chars {
			i = pos
			buf[i] = uint8(ch)
			if ch == 0 {
				break
			}
		}
		return unameField{string(buf[:i]), i >= 64}
	}

	return &uname{
		osName:    convert(&utsname.Sysname),
		nodeName:  convert(&utsname.Nodename),
		osRelease: convert(&utsname.Release),
		osVersion: convert(&utsname.Version),
		arch:      convert(&utsname.Machine),
	}
}
