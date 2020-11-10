package v1beta1

// ClusterHelmAddons specifies settings for cluster helm based addons
type ClusterHelmAddons struct {
	Repositories []Repository `yaml:"repositories"`
	Addons       []Addon      `yaml:"charts"`
}

// Addon single helm addon
type Addon struct {
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
