package fakes

import boshsettings "github.com/cloudfoundry/bosh-agent/settings"

type FakeSettingsSource struct {
	PublicKey    string
	PublicKeyErr error

	SettingsValue boshsettings.Settings
	SettingsErr   error

	DynamicSettings func() (boshsettings.Settings, error)
}

func (s FakeSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return s.PublicKey, s.PublicKeyErr
}

func (s FakeSettingsSource) Settings() (boshsettings.Settings, error) {
	if s.DynamicSettings != nil {
		return s.DynamicSettings()
	} else {
		return s.SettingsValue, s.SettingsErr
	}
}

func FailAfter(settings boshsettings.Settings, err error, after int) func() (boshsettings.Settings, error) {
	counter := 0
	return func() (boshsettings.Settings, error) {
		counter += 1
		if counter < after {
			return settings, err
		} else {
			return settings, nil
		}
	}
}
