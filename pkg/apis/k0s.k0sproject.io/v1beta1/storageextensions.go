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
