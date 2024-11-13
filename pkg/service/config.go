package service

// Config describes a configuration for a system service
type Config struct {
	Name         string
	DisplayName  string
	Description  string
	Arguments    []string
	Option       map[string]any
	Dependencies []string
}
