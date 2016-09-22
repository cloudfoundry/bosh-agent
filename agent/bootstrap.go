package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"

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

	publicKey, err := boot.settingsService.PublicSSHKeyForUsername(boshsettings.VCAPUsername)
	if err != nil {
		return bosherr.WrapError(err, "Setting up ssh: Getting public key")
	}

	if len(publicKey) > 0 {
		if err = boot.platform.SetupSSH(publicKey, boshsettings.VCAPUsername); err != nil {
			return bosherr.WrapError(err, "Setting up ssh")
		}
	}

	if err = boot.settingsService.LoadSettings(); err != nil {
		return bosherr.WrapError(err, "Fetching settings")
	}

	settings := boot.settingsService.GetSettings()

	if err = boot.setUserPasswords(settings.Env); err != nil {
		return bosherr.WrapError(err, "Settings user password")
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

	if err = boot.platform.SetupLogDir(); err != nil {
		return bosherr.WrapError(err, "Setting up log dir")
	}

	if err = boot.platform.SetupLoggingAndAuditing(); err != nil {
		return bosherr.WrapError(err, "Starting up logging and auditing utilities")
	}

	if err = boot.platform.SetupDataDir(); err != nil {
		return bosherr.WrapError(err, "Setting up data dir")
	}

	if err = boot.platform.SetupTmpDir(); err != nil {
		return bosherr.WrapError(err, "Setting up tmp dir")
	}

	if err = boot.platform.SetupHomeDir(); err != nil {
		return bosherr.WrapError(err, "Setting up home dir")
	}

	// check if settings.json and update_settings.json match
	//settings.Disks.Persistent vs updateSettings.DiskAssociations

	if err = boot.comparePersistentDisk(); err != nil {
		return bosherr.WrapError(err, "Comparing persistent disks")
	}

	for diskID := range settings.Disks.Persistent {
		diskSettings, _ := settings.PersistentDiskSettings(diskID)

		isPartitioned, err := boot.platform.IsPersistentDiskMountable(diskSettings)
		if err != nil {
			return bosherr.WrapError(err, "Checking if persistent disk is partitioned")
		}

		if isPartitioned {
			if err = boot.platform.MountPersistentDisk(diskSettings, boot.dirProvider.StoreDir()); err != nil {
				return bosherr.WrapError(err, "Mounting persistent disk")
			}
		}
	}

	if err = boot.platform.SetupMonitUser(); err != nil {
		return bosherr.WrapError(err, "Setting up monit user")
	}

	if err = boot.platform.StartMonit(); err != nil {
		return bosherr.WrapError(err, "Starting monit")
	}

	if settings.Env.GetRemoveDevTools() {
		packageFileListPath := path.Join(boot.dirProvider.EtcDir(), "dev_tools_file_list")

		if !boot.fs.FileExists(packageFileListPath) {
			return nil
		}

		if err = boot.platform.RemoveDevTools(packageFileListPath); err != nil {
			return bosherr.WrapError(err, "Removing Development Tools Packages")
		}
	}

	return nil
}

func (boot bootstrap) comparePersistentDisk() error {
	settings := boot.settingsService.GetSettings()
	updateSettingsPath := filepath.Join(boot.platform.GetDirProvider().BoshDir(), "update_settings.json")

	if boot.platform.GetFs().FileExists(updateSettingsPath) {

		contents, _ := boot.platform.GetFs().ReadFile(updateSettingsPath)
		var updateSettings boshsettings.UpdateSettings
		json.Unmarshal(contents, &updateSettings)

		for _, diskAssociation := range updateSettings.DiskAssociations {
			if _, ok := settings.PersistentDiskSettings(diskAssociation.DiskCID); !ok {
				return errors.New(fmt.Sprintf("Disk %s is not attached", diskAssociation.DiskCID))
			}
		}

		if len(settings.Disks.Persistent) > 1 {
			if len(settings.Disks.Persistent) > len(updateSettings.DiskAssociations) {
				return errors.New("Unexpected disk attached")
			}
		}

		return nil
	}

	if len(settings.Disks.Persistent) != 0 {
		return errors.New("Persistent disk count and disk association count mismatch")
	}

	return nil

	// updateSettingsPath := filepath.Join(boot.platform.GetDirProvider().BoshDir(), "update_settings.json")

	// boot.platform..GetFs().FileExists(updateSettingsPath)

	// contents, err := boot.platform.GetFs().ReadFile(updateSettingsPath)
	// if err != nil {
	// 	return err
	// }
	// var updateSettings boshsettings.UpdateSettings
	// json.Unmarshal(contents, &updateSettings)

	// settings := boot.settingsService.GetSettings()
	// //"persistent":{"vol-03a309d6":"/dev/sdf","vol-5b14a48e":"/dev/sdi","vol-a5b41e70":"/dev/sdh","vol-f7a00a22":"/dev/sdg"}
	// //"disk_associations":[{"name":"new-disk1","cid":"vol-03a309d6"},{"name":"new-disk2","cid":"vol-f7a00a22"},{"name":"new-disk3","cid":"vol-a5b41e70"},{"name":"new-disk4","cid":"vol-5b14a48e"}
	// if len(settings.Disks.Persistent) != len(updateSettings.DiskAssociations) {
	// 	return errors.New("Persistent disk count and disk association count mismatch")
	// }

	// for _, diskAssociation := range updateSettings.DiskAssociations {
	// 	// TODO change to use PersistentDiskSettings method
	// 	if _, ok := settings.Disks.Persistent[diskAssociation.DiskCID]; !ok {
	// 		return errors.New(fmt.Sprintf("Disk %s is not attached", diskAssociation.DiskCID))
	// 	}
	// }

	// return nil
}

func (boot bootstrap) setUserPasswords(env boshsettings.Env) error {
	password := env.GetPassword()
	if password == "" {
		return nil
	}

	if !env.GetKeepRootPassword() {
		err := boot.platform.SetUserPassword(boshsettings.RootUsername, password)
		if err != nil {
			return bosherr.WrapError(err, "Setting root password")
		}
	}

	err := boot.platform.SetUserPassword(boshsettings.VCAPUsername, password)
	if err != nil {
		return bosherr.WrapError(err, "Setting vcap password")
	}

	return nil
}
