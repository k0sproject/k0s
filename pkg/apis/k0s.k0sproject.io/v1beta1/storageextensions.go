/*
Copyright 2021 k0s authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1beta1

import "fmt"

// StorageExtenstion specifies cluster default storage
type StorageExtension struct {
	Type                      string `json:"type"`
	CreateDefaultStorageClass bool   `json:"create_default_storage_class"`
}

var _ Validateable = (*StorageExtension)(nil)

const (
	ExternalStorage = "external_storage"
	OpenEBSLocal    = "openebs_local_storage"
)

func (se *StorageExtension) Validate() []error {
	var errs []error
	switch se.Type {
	case ExternalStorage, OpenEBSLocal:
		// do nothing on valid types
	default:
		errs = append(errs, fmt.Errorf("unknown storage mode `%s`", se.Type))
	}
	if se.CreateDefaultStorageClass && se.Type == ExternalStorage {
		errs = append(errs, fmt.Errorf("can't create default storage class for external_storage"))
	}
	return errs
}
