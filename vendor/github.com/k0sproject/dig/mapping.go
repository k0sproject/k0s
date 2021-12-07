// Package dig provides a map[string]interface{} Mapping type that has ruby-like "dig" functionality.
//
// It can be used for example to access and manipulate arbitary nested YAML/JSON structures.
package dig

import "fmt"

// Mapping is a nested key-value map where the keys are strings and values are interface{}. In Ruby it is called a Hash (with string keys), in YAML it's called a "mapping".
type Mapping map[string]interface{}

// UnmarshalYAML for supporting yaml.Unmarshal
func (m *Mapping) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var result map[interface{}]interface{}
	if err := unmarshal(&result); err != nil {
		return err
	}
	*m = cleanUpInterfaceMap(result)
	return nil
}

// Dig is a simplistic implementation of a Ruby-like Hash.dig functionality.
//
// It returns a value from a (deeply) nested tree structure.
func (m *Mapping) Dig(keys ...string) interface{} {
	v, ok := (*m)[keys[0]]
	if !ok {
		return nil
	}
	switch v := v.(type) {
	case Mapping:
		if len(keys) == 1 {
			return v
		}
		return v.Dig(keys[1:]...)
	default:
		if len(keys) > 1 {
			return nil
		}
		return v
	}
}

// DigString is like Dig but returns the value as string
func (m *Mapping) DigString(keys ...string) string {
	v := m.Dig(keys...)
	val, ok := v.(string)
	if !ok {
		return ""
	}
	return val
}

// DigMapping always returns a mapping, creating missing or overwriting non-mapping branches in between
func (m *Mapping) DigMapping(keys ...string) Mapping {
	k := keys[0]
	cur := (*m)[k]
	switch v := cur.(type) {
	case Mapping:
		if len(keys) > 1 {
			return v.DigMapping(keys[1:]...)
		}
		return v
	default:
		n := Mapping{}
		(*m)[k] = n
		if len(keys) > 1 {
			return n.DigMapping(keys[1:]...)
		}
		return n
	}
}

// Cleans up a slice of interfaces into slice of actual values
func cleanUpInterfaceArray(in []interface{}) []interface{} {
	result := make([]interface{}, len(in))
	for i, v := range in {
		result[i] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the map keys to be strings
func cleanUpInterfaceMap(in map[interface{}]interface{}) Mapping {
	result := make(Mapping)
	for k, v := range in {
		result[fmt.Sprintf("%v", k)] = cleanUpMapValue(v)
	}
	return result
}

// Cleans up the value in the map, recurses in case of arrays and maps
func cleanUpMapValue(v interface{}) interface{} {
	switch v := v.(type) {
	case []interface{}:
		return cleanUpInterfaceArray(v)
	case map[interface{}]interface{}:
		return cleanUpInterfaceMap(v)
	case string, int, bool:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}
