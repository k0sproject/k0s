package install

// no need to build unix specific funcs into windows
func cleanupMount(path string)            {}
func cleanupNetworkNamespace(path string) {}
