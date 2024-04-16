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
	"bufio"
	"compress/gzip"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/k0sproject/k0s/internal/pkg/sysinfo/probes"
)

func (l *LinuxProbes) AssertKernelRelease(assert func(string) string) {
	l.Set("kernelRelease", func(path probes.ProbePath, current probes.Probe) probes.Probe {
		return probes.ProbeFn(func(r probes.Reporter) error {
			desc := probes.NewProbeDesc("Linux kernel release", path)
			//revive:disable:indent-error-flow
			if uname, err := l.probeUname(); err != nil {
				return r.Error(desc, err)
			} else if uname.osRelease.truncated {
				return r.Error(desc, errors.New(uname.osRelease.String()))
			} else if msg := assert(uname.osRelease.value); msg != "" {
				return r.Warn(desc, uname.osRelease, msg)
			} else {
				return r.Pass(desc, uname.osRelease)
			}
		})
	})
}

func (l *LinuxProbes) RequireKernelConfig(config, desc string, alternativeConfigs ...string) *KernelConfigProbes {
	return probeKConfig(l, l.probeKConfig, true, config, desc, alternativeConfigs...)
}

func (l *LinuxProbes) AssertKernelConfig(config, desc string, alternativeConfigs ...string) *KernelConfigProbes {
	return probeKConfig(l, l.probeKConfig, false, config, desc, alternativeConfigs...)
}

type KernelConfigProbes struct {
	probes.Probes

	path        probes.ProbePath
	probeConfig kConfigProber
}

func (k *KernelConfigProbes) RequireKernelConfig(config, desc string, alternativeConfigs ...string) *KernelConfigProbes {
	return probeKConfig(k, k.probeConfig, true, config, desc, alternativeConfigs...)
}

func (k *KernelConfigProbes) AssertKernelConfig(config, desc string, alternativeConfigs ...string) *KernelConfigProbes {
	return probeKConfig(k, k.probeConfig, false, config, desc, alternativeConfigs...)
}

// revive:disable:var-naming

type kConfigSpec struct {
	kConfig
	desc                string
	alternativeKConfigs []kConfig
	require             bool
}

func probeKConfig(
	parent probes.ParentProbe, probeConfig kConfigProber,
	require bool, config, desc string, alternativeConfigs ...string,
) *KernelConfigProbes {
	spec := &kConfigSpec{ensureKConfig(config), desc, nil, require}
	for _, alternativeConfig := range alternativeConfigs {
		spec.alternativeKConfigs = append(spec.alternativeKConfigs, ensureKConfig(alternativeConfig))
	}

	var kp *kConfigProbe
	parent.Set(config, func(path probes.ProbePath, current probes.Probe) probes.Probe {
		if probe, ok := current.(*kConfigProbe); ok {
			kp = probe
			kp.kConfigSpec = spec
		} else {
			kp = &kConfigProbe{&KernelConfigProbes{probes.NewProbesAtPath(path), path, probeConfig}, spec}
		}
		return kp
	})

	return kp.KernelConfigProbes
}

// https://github.com/torvalds/linux/blob/v4.3/Documentation/kbuild/kconfig-language.txt

type kConfigOption string

const (
	kConfigUnknown  kConfigOption = ""
	kConfigBuiltIn  kConfigOption = "y"
	kConfigAsModule kConfigOption = "m"
	kConfigLeftOut  kConfigOption = "n"
)

func (v kConfigOption) String() string {
	switch v {
	case kConfigBuiltIn:
		return "built-in"
	case kConfigAsModule:
		return "module"
	case kConfigLeftOut:
		return "left out"
	case kConfigUnknown:
		return "unknown"
	}

	return fmt.Sprintf("??? %q", string(v))
}

type kConfigProber func(config kConfig) (kConfigOption, error)

type KernelChecks struct {
	probeUname   unameProber
	probeKConfig kConfigProber
}

func NewKernelChecks() *KernelChecks {
	probeUname := newUnameProber()
	probeConfig := newKConfigProber(probeUname)
	return &KernelChecks{probeUname, probeConfig}
}

type kConfigs map[kConfig]kConfigOption

func newKConfigProber(probeUname unameProber) kConfigProber {
	var once sync.Once
	var kConfigs kConfigs
	var kConfigsErr error

	return func(config kConfig) (kConfigOption, error) {
		once.Do(func() {
			var u *uname
			u, kConfigsErr = probeUname()
			if kConfigsErr == nil {
				kConfigs, kConfigsErr = loadKConfigs(u.osRelease.value)
			}
		})

		return kConfigs[config], kConfigsErr
	}
}

const validKConfig = "[A-Z0-9_]+"

var validKConfigRegex = regexp.MustCompile("^" + validKConfig + "$")

type kConfig string

func ensureKConfig(config string) kConfig {
	if !validKConfigRegex.MatchString(config) {
		panic(fmt.Sprintf("invalid kernel config: %q", config))
	}

	return kConfig(config)
}

func (c kConfig) String() string {
	return fmt.Sprintf("CONFIG_%s", string(c))
}

type kConfigProbe struct {
	*KernelConfigProbes
	*kConfigSpec
}

func (k *kConfigProbe) Path() probes.ProbePath {
	return k.path
}

func (k *kConfigProbe) DisplayName() string {
	var buf strings.Builder
	buf.WriteString(k.kConfig.String())
	if k.desc != "" {
		buf.WriteString(": ")
		buf.WriteString(k.desc)
	}

	return buf.String()
}

func (k *kConfigProbe) Probe(reporter probes.Reporter) error {
	option, err := k.probeConfig(k.kConfig)
	if err != nil {
		var notFoundErr *noKConfigsFound
		if errors.As(err, &notFoundErr) {
			return reporter.Warn(k, notFoundErr, "")
		}
		return reporter.Error(k, err)
	}

	if err := k.probe(reporter, option); err != nil {
		return err
	}

	return k.Probes.Probe(reporter)
}

func (k *kConfigProbe) probe(reporter probes.Reporter, option kConfigOption) error {
	switch option {
	case kConfigBuiltIn, kConfigAsModule:
		return reporter.Pass(k, option)
	}

	var alsoTried []string
	for _, kConfig := range k.alternativeKConfigs {
		alsoTried = append(alsoTried, kConfig.String())
		altOption, err := k.probeConfig(k.kConfig)
		if err != nil {
			return reporter.Error(k, err)
		}

		switch altOption {
		case kConfigBuiltIn, kConfigAsModule:
			return reporter.Pass(k, &altKConfigOption{altOption, kConfig})
		}
	}

	msg := ""
	if len(k.alternativeKConfigs) > 0 {
		msg = fmt.Sprintf("also tried %s", strings.Join(alsoTried, ", "))
	}

	if k.require {
		return reporter.Reject(k, &option, msg)
	}

	return reporter.Warn(k, &option, msg)
}

type altKConfigOption struct {
	kConfigOption
	kConfig
}

func (a *altKConfigOption) String() string {
	return fmt.Sprintf("%s (via %s)", a.kConfigOption, &a.kConfig)
}

type noKConfigsFound struct {
	kernelRelease string
	checkedPaths  []string
}

func (n *noKConfigsFound) String() string {
	return "no kernel config found"
}

func (n *noKConfigsFound) Error() string {
	return fmt.Sprintf(
		"%s for kernel release %s in %s",
		n.String(), n.kernelRelease, strings.Join(n.checkedPaths, ", "),
	)
}

// loadKConfigs checks a list of well-known file system paths for kernel
// configuration files and tries to parse them.
func loadKConfigs(kernelRelease string) (kConfigs, error) {
	// At least some references to those paths may be fond here:
	// https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L794
	// https://github.com/torvalds/linux/blob/v4.3/init/Kconfig#L9
	possiblePaths := []string{
		"/proc/config.gz",
		"/boot/config-" + kernelRelease,
		"/usr/src/linux-" + kernelRelease + "/.config",
		"/usr/src/linux/.config",
		"/usr/lib/modules/" + kernelRelease + "/config",
		"/usr/lib/ostree-boot/config-" + kernelRelease,
		"/usr/lib/kernel/config-" + kernelRelease,
		"/usr/src/linux-headers-" + kernelRelease + "/.config",
		"/lib/modules/" + kernelRelease + "/build/.config",
	}

	for _, path := range possiblePaths {
		// open file for reading
		f, err := os.Open(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		defer f.Close()

		r := io.Reader(bufio.NewReader(f))

		// This is a gzip file (config.gz), unzip it.
		if filepath.Ext(path) == ".gz" {
			gr, err := gzip.NewReader(r)
			if err != nil {
				return nil, err
			}
			defer gr.Close()
			r = gr
		}

		return parseKConfigs(r)
	}

	return nil, &noKConfigsFound{kernelRelease, possiblePaths}
}

// parseKConfigs parses `r` line by line, extracting all kernel config options.
func parseKConfigs(r io.Reader) (kConfigs, error) {
	configs := kConfigs{}
	kConfigLineRegex := regexp.MustCompile(fmt.Sprintf(
		"^[\\s\\p{Zs}]*CONFIG_(%s)=([%s%s%s])",
		validKConfig, string(kConfigBuiltIn), string(kConfigLeftOut), string(kConfigAsModule),
	))
	s := bufio.NewScanner(r)
	for s.Scan() {
		if err := s.Err(); err != nil {
			return nil, err
		}

		if matches := kConfigLineRegex.FindStringSubmatch(s.Text()); matches != nil {
			configs[ensureKConfig(matches[1])] = kConfigOption(matches[2])
		}
	}
	return configs, nil
}
