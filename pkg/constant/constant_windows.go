//go:build windows
// +build windows

/*
Copyright 2020 k0s authors

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

package constant

const (
	// DataDirDefault folder contains all k0s state
	DataDirDefault = "C:\\var\\lib\\k0s"
	// CertRootDir defines the root location for all pki related artifacts
	CertRootDir = "C:\\var\\lib\\k0s\\pki"
	// BinDir defines the location for all pki related binaries
	BinDir = "C:\\var\\lib\\k0s\\bin"
	// RunDir run directory
	RunDir = "C:\\run\\k0s"
	// ManifestsDir stack applier directory
	ManifestsDir = "C:\\var\\lib\\k0s\\manifests"
	// KubeletVolumePluginDir defines the location for kubelet plugins volume executables
	KubeletVolumePluginDir = "C:\\usr\\libexec\\k0s\\kubelet-plugins\\volume\\exec"

	KineSocket                     = "kine\\kine.sock:2379"
	KubePauseContainerImage        = "mcr.microsoft.com/oss/kubernetes/pause"
	KubePauseContainerImageVersion = "1.4.1"
	K0sConfigPathDefault           = "C:\\etc\\k0s\\k0s.yaml"
	RuntimeConfigPathDefault       = RunDir + "\\k0s.yaml"
)
