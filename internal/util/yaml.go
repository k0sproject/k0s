package util

import (
	"fmt"
	"strings"

	"sigs.k8s.io/yaml"
)

// var fieldNamePattern = regexp.MustCompile("field ([^ ]+)")

// YamlUnmarshalStrictIgnoringFields does UnmarshalStrict but ignores type errors for given fields
func YamlUnmarshalStrictIgnoringFields(in []byte, out interface{}, ignore []string) (err error) {
	err = yaml.UnmarshalStrict(in, out)
	if err != nil {
		// parse errors for unknown field errors
		for _, field := range ignore {
			unknownFieldErr := fmt.Sprintf("unknown field \"%s\"", field)
			if strings.Contains(err.Error(), unknownFieldErr) {
				// reset err on unknown masked fields
				err = nil
			}
		}
		// we have some other error
		return err
	}
	return nil
}
