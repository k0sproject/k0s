package build

// Version gets overridden at build time using -X main.Version=$VERSION
var Version string

var RuncVersion string
var ContainerdVersion string
var KubernetesVersion string
var KineVersion string
var EtcdVersion string
var KonnectivityVersion string
