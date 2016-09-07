package action

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

type DiskAssociations struct {
	Associations []DiskAssociation `json:"disk_associations"`
}

type DiskAssociation struct {
	Name    string `json:"name"`
	DiskCID string `json:"cid"`
}

type AssociateDisksAction struct {
	settingsService boshsettings.Service
	platform        boshplatform.Platform
	logger          boshlog.Logger
}

func NewAssociateDisks(
	settingsService boshsettings.Service,
	platform boshplatform.Platform,
	logger boshlog.Logger,
) AssociateDisksAction {
	return AssociateDisksAction{
		settingsService: settingsService,
		logger:          logger,
		platform:        platform,
	}
}

func (a AssociateDisksAction) Run(diskAssociations DiskAssociations) (string, error) {
	err := a.settingsService.LoadSettings()
	if err != nil {
		return "", err
	}

	settings := a.settingsService.GetSettings()

	for _, diskAssociation := range diskAssociations.Associations {
		diskSettings, found := settings.PersistentDiskSettings(diskAssociation.DiskCID)
		if !found {
			return "", bosherr.Errorf("Persistent disk settings contains no disk with CID: %s", diskAssociation.DiskCID)
		}

		err := a.platform.AssociateDisk(diskAssociation.Name, diskSettings)
		if err != nil {
			return "", err
		}
	}

	return "associated", nil
}

func (a AssociateDisksAction) IsAsynchronous() bool {
	return true
}

func (a AssociateDisksAction) IsPersistent() bool {
	return false
}

func (a AssociateDisksAction) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a AssociateDisksAction) Cancel() error {
	return errors.New("not supported")
}
