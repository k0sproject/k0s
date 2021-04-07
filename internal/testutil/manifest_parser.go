package testutil

import (
	"bytes"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

func ParseManifests(data []byte) ([]*unstructured.Unstructured, error) {
	resources := []*unstructured.Unstructured{}

	decoder := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var resource map[string]interface{}
	for decoder.Decode(&resource) == nil {
		item := &unstructured.Unstructured{
			Object: resource,
		}
		if item.GetAPIVersion() != "" && item.GetKind() != "" {
			resources = append(resources, item)
			resource = nil
		}
	}

	return resources, nil

}
