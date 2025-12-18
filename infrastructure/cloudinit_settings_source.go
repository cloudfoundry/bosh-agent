package infrastructure

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type CloudInitSettingsSource struct {
	platform  boshplatform.Platform
	cmdRunner boshsys.CmdRunner

	logTag string
	logger boshlog.Logger
}

func NewCloudInitSettingsSource(
	platform boshplatform.Platform,
	logger boshlog.Logger,
) *CloudInitSettingsSource {
	return &CloudInitSettingsSource{
		platform:  platform,
		cmdRunner: platform.GetRunner(),
		logTag:    "CloudInitSettingsSource",
		logger:    logger,
	}
}

func (s CloudInitSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return "", nil
}

func (s *CloudInitSettingsSource) Settings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	// Try to get settings data from vmware-rpctool first, then fallback to vmtoolsd
	stdout, _, exitStatus, err := s.cmdRunner.RunCommand("vmware-rpctool", "info-get guestinfo.userdata")
	if err != nil || exitStatus != 0 {
		stdout, _, exitStatus, err = s.cmdRunner.RunCommand("vmtoolsd", "--cmd", "info-get guestinfo.userdata")
		if err != nil || exitStatus != 0 {
			return boshsettings.Settings{}, bosherr.WrapError(err, "getting user data from vmware tools")
		}
	}

	decodedBytes, err := base64.StdEncoding.DecodeString(stdout)
	if err != nil {
		return boshsettings.Settings{}, bosherr.WrapError(err, "decoding user data")
	}

	// unzip the data, if it is gzipped
	if bytes.HasPrefix(decodedBytes, []byte{0x1f, 0x8b}) {
		gzReader, err := gzip.NewReader(bytes.NewReader(decodedBytes))
		if err != nil {
			return boshsettings.Settings{}, bosherr.WrapError(err, "unzipping user data")
		}
		//nolint:errcheck
		defer gzReader.Close()

		decodedBytes, err = io.ReadAll(gzReader)
		if err != nil {
			return boshsettings.Settings{}, bosherr.WrapError(err, "unzipping user data")
		}
	}

	err = json.Unmarshal(decodedBytes, &settings)
	if err != nil {
		return settings, bosherr.WrapErrorf(
			err, "Parsing settings from vmware tools")
	}

	_, _, _, err = s.cmdRunner.RunCommand("vmware-rpctool", "info-set guestinfo.userdata ---")
	if err != nil {
		//nolint:errcheck
		s.cmdRunner.RunCommand("vmtoolsd", "--cmd", "info-set guestinfo.userdata ---")
	}
	_, _, _, err = s.cmdRunner.RunCommand("vmware-rpctool", "info-set guestinfo.userdata.encoding ")
	if err != nil {
		//nolint:errcheck
		s.cmdRunner.RunCommand("vmtoolsd", "--cmd", "info-set guestinfo.userdata.encoding ")
	}

	return settings, nil
}
