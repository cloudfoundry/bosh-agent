package action

import (
	"errors"

	"github.com/cloudfoundry/bosh-agent/v2/agent/utils"
	"github.com/cloudfoundry/bosh-agent/v2/platform"
	"github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"github.com/cloudfoundry/bosh-utils/logger"
)

type UpdateSettingsAction struct {
	agentKiller        utils.Killer
	trustedCertManager cert.Manager
	logger             logger.Logger
	settingsService    boshsettings.Service
	platform           platform.Platform
}

func NewUpdateSettings(service boshsettings.Service, platform platform.Platform, trustedCertManager cert.Manager, logger logger.Logger, agentKiller utils.Killer) UpdateSettingsAction {
	return UpdateSettingsAction{
		agentKiller:        agentKiller,
		trustedCertManager: trustedCertManager,
		logger:             logger,
		settingsService:    service,
		platform:           platform,
	}
}

func (a UpdateSettingsAction) IsAsynchronous(_ ProtocolVersion) bool {
	return true
}

func (a UpdateSettingsAction) IsPersistent() bool {
	return true
}

func (a UpdateSettingsAction) IsLoggable() bool {
	return true
}

func (a UpdateSettingsAction) Run(newUpdateSettings boshsettings.UpdateSettings) (string, error) {
	var restartNeeded bool
	err := a.settingsService.LoadSettings()
	if err != nil {
		return "", err
	}

	for _, diskAssociation := range newUpdateSettings.DiskAssociations {
		diskSettingsToAssociate, err := a.settingsService.GetPersistentDiskSettings(diskAssociation.DiskCID)
		if err != nil {
			return "", bosherr.WrapError(err, "Fetching disk settings")
		}

		err = a.platform.AssociateDisk(diskAssociation.Name, diskSettingsToAssociate)
		if err != nil {
			return "", err
		}
	}

	err = a.trustedCertManager.UpdateCertificates(newUpdateSettings.TrustedCerts)
	if err != nil {
		return "", err
	}

	existingSettings := a.settingsService.GetSettings().UpdateSettings
	restartNeeded = existingSettings.MergeSettings(newUpdateSettings)
	err = a.settingsService.SaveUpdateSettings(existingSettings)
	if err != nil {
		return "", err
	}

	if restartNeeded {
		a.agentKiller.KillAgent()
		panic("This line of code should be unreachable due to killing of agent")
	}

	return "ok", nil
}

func (a UpdateSettingsAction) Resume() (interface{}, error) {
	return "ok", nil
}

func (a UpdateSettingsAction) Cancel() error {
	return errors.New("not supported")
}
