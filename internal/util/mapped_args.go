package util

import "fmt"

// MappedArgs defines map like arguments that can be "evaluated" into args=value pairs
type MappedArgs map[string]string

// ToArgs maps the data into cmd arguments like foo=bar baz=baf
func (m MappedArgs) ToArgs() []string {
	args := make([]string, len(m))
	idx := 0
	for k, v := range m {
		args[idx] = fmt.Sprintf("%s=%s", k, v)
		idx++
	}
	return args
}

// Merge merges two maps together
func (m MappedArgs) Merge(other MappedArgs) {
	if len(other) > 0 {
		for k, v := range other {
			m[k] = v
		}
	}
}
