package infrastructure

import (
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type SettingsSource interface {
	PublicSSHKeyForUsername(string) (string, error)
	Settings() (boshsettings.Settings, error)
}
