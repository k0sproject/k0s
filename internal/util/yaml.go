package util

import (
	"gopkg.in/yaml.v2"
	"regexp"
	"strings"
)

var fieldNamePattern = regexp.MustCompile("field ([^ ]+)")

// YamlUnmarshalStrictIgnoringFields does UnmarshalStrict but ignores type errors for given fields
func YamlUnmarshalStrictIgnoringFields(in []byte, out interface{}, ignore []string) (err error) {
	err = yaml.UnmarshalStrict(in, out)
	if err == nil {
		return nil
	}
	errYaml, isTypeError := err.(*yaml.TypeError)
	if !isTypeError {
		return err
	}
	for _, fieldErr := range errYaml.Errors {
		if !strings.Contains(fieldErr, "not found in type") {
			// we have some other error, just return error message
			return errYaml
		}
		match := fieldNamePattern.FindStringSubmatch(fieldErr)
		if match == nil {
			// again some other error
			return errYaml
		}

		if StringSliceContains(ignore, match[1]) {
			continue
		}
		// we have type error but not for the masked fields, return error
		return errYaml
	}

	return nil
}
