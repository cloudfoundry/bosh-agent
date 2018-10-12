package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type FakeSettingsService struct {
	PublicKey    string
	PublicKeyErr error

	LoadSettingsError  error
	SettingsWereLoaded bool

	GetPersistentDiskSettingsError    error
	GetAllPersistentDiskSettingsError error
	PersistentDiskSettingsWereLoaded  bool

	RemovePersistentDiskSettingsError error

	InvalidateSettingsError error
	SettingsWereInvalidated bool

	PersistentDiskSettings map[string]boshsettings.DiskSettings
	Settings               boshsettings.Settings

	GetPersistentDiskSettingsCallCount    int
	RemovePersistentDiskSettingsCallCount int
	RemovePersistentDiskSettingsLastArg   string
	SavePersistentDiskSettingsCallCount   int
	SavePersistentDiskSettingsErr         error
	SavePersistentDiskSettingsLastArg     boshsettings.DiskSettings
}

func (service *FakeSettingsService) InvalidateSettings() error {
	service.SettingsWereInvalidated = true
	return service.InvalidateSettingsError
}

func (service *FakeSettingsService) PublicSSHKeyForUsername(_ string) (string, error) {
	return service.PublicKey, service.PublicKeyErr
}

func (service *FakeSettingsService) LoadSettings() error {
	service.SettingsWereLoaded = true
	return service.LoadSettingsError
}

func (service FakeSettingsService) GetSettings() boshsettings.Settings {
	return service.Settings
}

func (service *FakeSettingsService) GetPersistentDiskSettings(diskCID string) (boshsettings.DiskSettings, error) {
	service.GetPersistentDiskSettingsCallCount++
	service.PersistentDiskSettingsWereLoaded = true
	return service.PersistentDiskSettings[diskCID], service.GetPersistentDiskSettingsError
}

func (service *FakeSettingsService) RemovePersistentDiskSettings(diskID string) error {
	service.RemovePersistentDiskSettingsLastArg = diskID
	service.RemovePersistentDiskSettingsCallCount++
	return service.RemovePersistentDiskSettingsError
}

func (service *FakeSettingsService) SavePersistentDiskSettings(settings boshsettings.DiskSettings) error {
	service.SavePersistentDiskSettingsCallCount++
	service.SavePersistentDiskSettingsLastArg = settings
	if service.SavePersistentDiskSettingsErr != nil {
		return service.SavePersistentDiskSettingsErr
	}
	return nil
}

func (service *FakeSettingsService) GetAllPersistentDiskSettings() (map[string]boshsettings.DiskSettings, error) {
	return service.PersistentDiskSettings, service.GetAllPersistentDiskSettingsError
}
