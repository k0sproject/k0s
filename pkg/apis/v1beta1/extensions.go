package v1beta1

// ClusterExtensions specifies cluster extensions
type ClusterExtensions struct {
	Helm *HelmExtensions `yaml:"helm"`
}

// HelmExtensions specifies settings for cluster helm based extensions
type HelmExtensions struct {
	Repositories []Repository `yaml:"repositories"`
	Charts       []Chart      `yaml:"charts"`
}

// Chart single helm addon
type Chart struct {
	Name      string `yaml:"name"`
	ChartName string `yaml:"chartname"`
	Version   string `yaml:"version"`
	Values    string `yaml:"values"`
	TargetNS  string `yaml:"namespace"`
}

// Repository describes single repository entry. Fields map to the CLI flags for the "helm add" command
type Repository struct {
	Name     string `yaml:"name"`
	URL      string `yaml:"url"`
	CAFile   string `yaml:"caFile"`
	CertFile string `yaml:"certFile"`
	Insecure bool   `yaml:"insecure"`
	KeyFile  string `yaml:"keyfile"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}
