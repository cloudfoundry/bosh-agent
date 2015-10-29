package agent

import (
	"errors"

	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Bootstrap interface {
	Run() error
}

type bootstrap struct {
	fs              boshsys.FileSystem
	platform        boshplatform.Platform
	dirProvider     boshdir.Provider
	settingsService boshsettings.Service
	logger          boshlog.Logger
}

func NewBootstrap(
	platform boshplatform.Platform,
	dirProvider boshdir.Provider,
	settingsService boshsettings.Service,
	logger boshlog.Logger,
) Bootstrap {
	return bootstrap{
		fs:              platform.GetFs(),
		platform:        platform,
		dirProvider:     dirProvider,
		settingsService: settingsService,
		logger:          logger,
	}
}

func (boot bootstrap) Run() (err error) {
	if err = boot.platform.SetupRuntimeConfiguration(); err != nil {
		return bosherr.WrapError(err, "Setting up runtime configuration")
	}
        //  delete public key setting temporarily

	settings := boot.settingsService.GetSettings()

	if err = boot.setUserPasswords(settings.Env); err != nil {
		return bosherr.WrapError(err, "Settings user password")
	}

	if err = boot.setRootPasswords(settings.Env); err != nil {
		return bosherr.WrapError(err, "Settings root password")
	}

	if err = boot.platform.SetupHostname(settings.AgentID); err != nil {
		return bosherr.WrapError(err, "Setting up hostname")
	}

	if err = boot.platform.SetupNetworking(settings.Networks); err != nil {
		return bosherr.WrapError(err, "Setting up networking")
	}

	if err = boot.platform.SetTimeWithNtpServers(settings.Ntp); err != nil {
		return bosherr.WrapError(err, "Setting up NTP servers")
	}

	if err = boot.platform.SetupRawEphemeralDisks(settings.RawEphemeralDiskSettings()); err != nil {
		return bosherr.WrapError(err, "Setting up raw ephemeral disk")
	}

	ephemeralDiskPath := boot.platform.GetEphemeralDiskPath(settings.EphemeralDiskSettings())
	if err = boot.platform.SetupEphemeralDiskWithPath(ephemeralDiskPath); err != nil {
		return bosherr.WrapError(err, "Setting up ephemeral disk")
	}

	if err = boot.platform.SetupRootDisk(ephemeralDiskPath); err != nil {
		return bosherr.WrapError(err, "Setting up root disk")
	}

	if err = boot.platform.SetupDataDir(); err != nil {
		return bosherr.WrapError(err, "Setting up data dir")
	}

	if err = boot.platform.SetupTmpDir(); err != nil {
		return bosherr.WrapError(err, "Setting up tmp dir")
	}

	if len(settings.Disks.Persistent) > 1 {
		return errors.New("Error mounting persistent disk, there is more than one persistent disk")
	}

	for diskID := range settings.Disks.Persistent {
		diskSettings, _ := settings.PersistentDiskSettings(diskID)
		if err = boot.platform.MountPersistentDisk(diskSettings, boot.dirProvider.StoreDir()); err != nil {
			return bosherr.WrapError(err, "Mounting persistent disk")
		}
	}

	if err = boot.platform.SetupMonitUser(); err != nil {
		return bosherr.WrapError(err, "Setting up monit user")
	}

	if err = boot.platform.StartMonit(); err != nil {
		return bosherr.WrapError(err, "Starting monit")
	}

	return nil
}

func (boot bootstrap) setUserPasswords(env boshsettings.Env) error {
	password := env.GetUserPassword()
	if password == "" {
		return nil
	}

	err := boot.platform.SetUserPassword(boshsettings.VCAPUsername, password)
	if err != nil {
		return bosherr.WrapError(err, "Setting vcap password")
	}

	return nil
}

func (boot bootstrap) setRootPasswords(env boshsettings.Env) error {
	password := env.GetRootPassword()
	if password == "" {
		return nil
	}

	err := boot.platform.SetUserPassword(boshsettings.RootUsername, password)
	if err != nil {
		return bosherr.WrapError(err, "Setting root password")
	}

	return nil
}
