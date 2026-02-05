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

const (
	defaultRpcToolPath  = "vmware-rpctool"
	defaultVmToolsdPath = "vmtoolsd"
)

type VsphereGuestInfoSettingsSource struct {
	platform  boshplatform.Platform
	cmdRunner boshsys.CmdRunner

	rpcToolPath  string
	vmToolsdPath string

	logTag string
	logger boshlog.Logger
}

func NewVsphereGuestInfoSettingsSource(
	platform boshplatform.Platform,
	logger boshlog.Logger,
	rpcToolPath string,
	vmToolsdPath string,
) *VsphereGuestInfoSettingsSource {
	if rpcToolPath == "" {
		rpcToolPath = defaultRpcToolPath
	}
	if vmToolsdPath == "" {
		vmToolsdPath = defaultVmToolsdPath
	}
	return &VsphereGuestInfoSettingsSource{
		platform:     platform,
		cmdRunner:    platform.GetRunner(),
		rpcToolPath:  rpcToolPath,
		vmToolsdPath: vmToolsdPath,
		logTag:       "VsphereGuestInfoSettingsSource",
		logger:       logger,
	}
}

func (s VsphereGuestInfoSettingsSource) PublicSSHKeyForUsername(string) (string, error) {
	return "", nil
}

func (s *VsphereGuestInfoSettingsSource) Settings() (boshsettings.Settings, error) {
	var settings boshsettings.Settings

	// Try to get settings data from vmware-rpctool first, then fallback to vmtoolsd
	stdout, _, exitStatus, err := s.vmWareRPC("info-get guestinfo.userdata")
	if err != nil || exitStatus != 0 {
		return boshsettings.Settings{}, bosherr.WrapError(err, "getting user data from vmware tools")
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

	_, _, _, err = s.vmWareRPC("info-set guestinfo.userdata ---")
	if err != nil {
		s.logger.Error("vsphere-guest-info-settings-source", "warning: error clearing guestinfo.userdata", "error", err)
	}

	_, _, _, err = s.vmWareRPC("info-set guestinfo.userdata.encoding ''")
	if err != nil {
		s.logger.Error("vsphere-guest-info-settings-source", "warning: error clearing guestinfo.userdata.encoding", "error", err)
	}

	return settings, nil
}

// vmWareRPC runs the given command using the configured rpctool path, and if it fails,
// it runs the same command using the configured vmtoolsd path.
// For some versions, vmware-rpctool is significantly faster than vmtoolsd, so we use it if it is available.
func (s *VsphereGuestInfoSettingsSource) vmWareRPC(cmd string) (string, string, int, error) {
	stdOut, stdErr, status, err := s.cmdRunner.RunCommand(s.rpcToolPath, cmd)
	if err != nil || status != 0 {
		return s.cmdRunner.RunCommand(s.vmToolsdPath, "--cmd", cmd)
	}
	return stdOut, stdErr, status, nil
}
