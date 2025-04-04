package settings

import (
	"reflect"
)

type UpdateSettings struct {
	Blobstores       []Blobstore      `json:"blobstores"`
	DiskAssociations DiskAssociations `json:"disk_associations"`
	Mbus             MBus             `json:"mbus"`
	TrustedCerts     string           `json:"trusted_certs"`
}

func (updateSettings *UpdateSettings) MergeSettings(newSettings UpdateSettings) bool {
	var mbusOrBlobstoreSettingsChanged bool

	updateSettings.TrustedCerts = newSettings.TrustedCerts
	updateSettings.DiskAssociations = newSettings.DiskAssociations

	if !reflect.DeepEqual(newSettings.Mbus, updateSettings.Mbus) && !reflect.DeepEqual(newSettings.Mbus, MBus{}) {
		updateSettings.Mbus = newSettings.Mbus
		mbusOrBlobstoreSettingsChanged = true
	}

	if !reflect.DeepEqual(newSettings.Blobstores, updateSettings.Blobstores) && newSettings.Blobstores != nil {
		updateSettings.Blobstores = newSettings.Blobstores
		mbusOrBlobstoreSettingsChanged = true
	}

	return mbusOrBlobstoreSettingsChanged
}
