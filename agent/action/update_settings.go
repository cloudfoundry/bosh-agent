package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/platform"
	"github.com/cloudfoundry/bosh-agent/platform/cert"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/logger"
)

type UpdateSettingsAction struct {
	trustedCertManager cert.Manager
	logger             logger.Logger
	settingsService    boshsettings.Service
	platform           platform.Platform
}

func NewUpdateSettings(service boshsettings.Service, platform platform.Platform, trustedCertManager cert.Manager, logger logger.Logger) UpdateSettingsAction {
	return UpdateSettingsAction{
		trustedCertManager: trustedCertManager,
		logger:             logger,
		settingsService:    service,
		platform:           platform,
	}
}

func (a UpdateSettingsAction) IsAsynchronous() bool {
	return true
}

func (a UpdateSettingsAction) IsPersistent() bool {
	return false
}

func (a UpdateSettingsAction) Run(newUpdateSettings boshsettings.UpdateSettings) (string, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return "", err
	}

	currentSettings := a.settingsService.GetSettings()

	for _, diskAssociation := range newUpdateSettings.DiskAssociations {
		diskSettings, found := currentSettings.PersistentDiskSettings(diskAssociation.DiskCID)
		if !found {
			return "", bosherr.Errorf("Persistent disk settings contains no disk with CID: %s", diskAssociation.DiskCID)
		}

		err := a.platform.AssociateDisk(diskAssociation.Name, diskSettings)
		if err != nil {
			return "", err
		}
	}

	err = a.trustedCertManager.UpdateCertificates(newUpdateSettings.TrustedCerts)
	if err != nil {
		return "", err
	}

	return "updated", nil
}

func (a UpdateSettingsAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a UpdateSettingsAction) Cancel() error {
	return errors.New("not supported")
}
