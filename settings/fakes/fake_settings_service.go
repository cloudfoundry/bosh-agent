package fakes

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type FakeSettingsService struct {
	PublicKey    string
	PublicKeyErr error

	LoadSettingsError  error
	SettingsWereLoaded bool

	GetPersistentDiskHintsError   error
	PersistentDiskHintsWereLoaded bool

	GetPersistentDiskHintResult boshsettings.DiskSettings
	GetPersistentDiskHintError  error
	PersistentDiskHintWasFound  bool

	RemovePersistentDiskHintsError error

	InvalidateSettingsError error
	SettingsWereInvalidated bool

	PersistentDiskHints map[string]boshsettings.DiskSettings
	Settings            boshsettings.Settings

	GetPersistentDiskHintsCallCount    int
	RemovePersistentDiskHintsCallCount int
	SavePersistentDiskHintCallCount    int
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

func (service *FakeSettingsService) GetPersistentDiskHints() (map[string]boshsettings.DiskSettings, error) {
	service.GetPersistentDiskHintsCallCount++
	service.PersistentDiskHintsWereLoaded = true
	return service.PersistentDiskHints, service.GetPersistentDiskHintsError
}

func (service *FakeSettingsService) GetPersistentDiskHint(diskID string) (boshsettings.DiskSettings, bool, error) {
	return service.GetPersistentDiskHintResult, service.PersistentDiskHintWasFound, service.GetPersistentDiskHintError
}

func (service *FakeSettingsService) RemovePersistentDiskHint(diskID string) error {
	service.RemovePersistentDiskHintsCallCount++
	return service.RemovePersistentDiskHintsError
}

func (service *FakeSettingsService) SavePersistentDiskHint(_ boshsettings.DiskSettings) error {
	service.SavePersistentDiskHintCallCount++
	return nil
}
