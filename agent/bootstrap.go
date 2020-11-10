package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"path/filepath"

	"github.com/cloudfoundry/bosh-agent/agent/applier/applyspec"
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
	specService     applyspec.V1Service
	logger          boshlog.Logger
	logTag          string
}

func NewBootstrap(
	platform boshplatform.Platform,
	dirProvider boshdir.Provider,
	settingsService boshsettings.Service,
	specService applyspec.V1Service,
	logger boshlog.Logger,
) Bootstrap {
	return bootstrap{
		fs:              platform.GetFs(),
		platform:        platform,
		dirProvider:     dirProvider,
		settingsService: settingsService,
		specService:     specService,
		logger:          logger,
		logTag:          "bootstrap",
	}
}

func (boot bootstrap) Run() (err error) {
	if err = boot.platform.SetupRuntimeConfiguration(); err != nil {
		return bosherr.WrapError(err, "Setting up runtime configuration")
	}

	iaasPublicKey, err := boot.settingsService.PublicSSHKeyForUsername(boshsettings.VCAPUsername)
	if err != nil {
		return bosherr.WrapError(err, "Setting up ssh: Getting iaas public key")
	}

	if len(iaasPublicKey) > 0 {
		if err = boot.platform.SetupSSH([]string{iaasPublicKey}, boshsettings.VCAPUsername); err != nil {
			return bosherr.WrapError(err, "Setting up iaas ssh")
		}
	}

	if err = boot.settingsService.LoadSettings(); err != nil {
		return bosherr.WrapError(err, "Fetching settings")
	}

	settings := boot.settingsService.GetSettings()

	envPublicKeys := settings.Env.GetAuthorizedKeys()

	if len(envPublicKeys) > 0 {
		publicKeys := envPublicKeys

		if len(iaasPublicKey) > 0 {
			publicKeys = append(publicKeys, iaasPublicKey)
		}

		if err = boot.platform.SetupSSH(publicKeys, boshsettings.VCAPUsername); err != nil {
			return bosherr.WrapError(err, "Adding env-configured ssh keys")
		}
	}

	if err = boot.setUserPasswords(settings.Env); err != nil {
		return bosherr.WrapError(err, "Settings user password")
	}

	if err = boot.platform.SetupIPv6(settings.Env.Bosh.IPv6); err != nil {
		return bosherr.WrapError(err, "Setting up IPv6")
	}

	if err = boot.platform.SetupHostname(settings.AgentID); err != nil {
		return bosherr.WrapError(err, "Setting up hostname")
	}

	if err = boot.platform.SetupNetworking(settings.Networks); err != nil {
		return bosherr.WrapError(err, "Setting up networking")
	}

	if err = boot.platform.SetupRawEphemeralDisks(settings.RawEphemeralDiskSettings()); err != nil {
		return bosherr.WrapError(err, "Setting up raw ephemeral disk")
	}

	ephemeralDiskPath := boot.platform.GetEphemeralDiskPath(settings.EphemeralDiskSettings())
	desiredSwapSizeInBytes := settings.Env.GetSwapSizeInBytes()
	if err = boot.platform.SetupEphemeralDiskWithPath(ephemeralDiskPath, desiredSwapSizeInBytes, settings.AgentID); err != nil {
		return bosherr.WrapError(err, "Setting up ephemeral disk")
	}

	if err = boot.platform.SetupRootDisk(ephemeralDiskPath); err != nil {
		return bosherr.WrapError(err, "Setting up root disk")
	}

	if err = boot.platform.SetupSharedMemory(); err != nil {
		return bosherr.WrapError(err, "Setting up Shared Memory")
	}

	if err = boot.platform.SetupLogDir(); err != nil {
		return bosherr.WrapError(err, "Setting up log dir")
	}

	if err = boot.platform.SetTimeWithNtpServers(settings.GetNtpServers()); err != nil {
		return bosherr.WrapError(err, "Setting up NTP servers")
	}

	if err = boot.platform.SetupLoggingAndAuditing(); err != nil {
		return bosherr.WrapError(err, "Starting up logging and auditing utilities")
	}

	if err = boot.platform.SetupDataDir(settings.Env.Bosh.JobDir, settings.Env.Bosh.RunDir); err != nil {
		return bosherr.WrapError(err, "Setting up data dir")
	}

	if err = boot.platform.SetupTmpDir(); err != nil {
		return bosherr.WrapError(err, "Setting up tmp dir")
	}

	if settings.TmpFSEnabled() {
		if err = boot.platform.SetupCanRestartDir(); err != nil {
			return bosherr.WrapError(err, "Setting up canrestart dir")
		}
	}

	if err = boot.platform.SetupHomeDir(); err != nil {
		return bosherr.WrapError(err, "Setting up home dir")
	}

	if err = boot.platform.SetupBlobsDir(); err != nil {
		return bosherr.WrapError(err, "Setting up blobs dir")
	}

	if err := boot.checkLastMountedCid(settings); err != nil {
		return bosherr.WrapError(err, "Checking last mounted CID")
	}

	if err = boot.mountLastMountedDisk(); err != nil {
		return bosherr.WrapError(err, "Mounting last mounted disk")
	}

	if err = boot.comparePersistentDisk(); err != nil {
		return bosherr.WrapError(err, "Comparing persistent disks")
	}

	v1Spec, err := boot.specService.Get()
	if err != nil {
		return bosherr.WrapError(err, "Cannot get v1spec from SpecService")
	}

	for _, job := range v1Spec.Jobs() {
		err = job.CreateDirectories(boot.fs, boot.dirProvider)
		if err != nil {
			return bosherr.WrapError(err, "Cannot create directories for jobs")
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

	if settings.Env.GetRemoveStaticLibraries() {
		staticLibrariesListPath := path.Join(boot.dirProvider.EtcDir(), "static_libraries_list")

		if !boot.fs.FileExists(staticLibrariesListPath) {
			return nil
		}

		if err = boot.platform.RemoveStaticLibraries(staticLibrariesListPath); err != nil {
			return bosherr.WrapError(err, "Removing static libraries")
		}
	}

	return nil
}

func (boot bootstrap) comparePersistentDisk() error {
	updateSettingsPath := filepath.Join(boot.platform.GetDirProvider().BoshDir(), "update_settings.json")

	var updateSettings boshsettings.UpdateSettings

	if boot.platform.GetFs().FileExists(updateSettingsPath) {
		contents, err := boot.platform.GetFs().ReadFile(updateSettingsPath)
		if err != nil {
			return bosherr.WrapError(err, "Reading update_settings.json")
		}

		if err = json.Unmarshal(contents, &updateSettings); err != nil {
			return bosherr.WrapError(err, "Unmarshalling update_settings.json")
		}
	}

	for _, diskAssociation := range updateSettings.DiskAssociations {
		_, err := boot.settingsService.GetPersistentDiskSettings(diskAssociation.DiskCID)
		if err != nil {
			return fmt.Errorf("Disk %s is not attached", diskAssociation.DiskCID)
		}
	}

	allSettings, err := boot.settingsService.GetAllPersistentDiskSettings()
	if err != nil {
		return errors.New("Reading all persistent disk settings")
	}

	if len(allSettings) > 1 && len(allSettings) > len(updateSettings.DiskAssociations) {
		return errors.New("Unexpected disk attached")
	}

	return nil
}

func (boot bootstrap) setUserPasswords(env boshsettings.Env) error {
	password := env.GetPassword()

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

func (boot bootstrap) checkLastMountedCid(settings boshsettings.Settings) error {
	lastMountedCid, err := boot.lastMountedCid()
	if err != nil {
		return bosherr.WrapError(err, "Fetching last mounted disk CID")
	}

	if lastMountedCid == "" {
		return nil
	}

	allDiskSettings, err := boot.settingsService.GetAllPersistentDiskSettings()
	if err != nil {
		return bosherr.WrapError(err, "Reading persistent disk settings")
	}

	if len(allDiskSettings) == 0 {
		return nil
	}

	_, err = boot.settingsService.GetPersistentDiskSettings(lastMountedCid)
	if err != nil {
		return bosherr.WrapError(err, "Attached disk disagrees with previous mount")
	}

	return nil
}

func (boot bootstrap) lastMountedCid() (string, error) {
	managedDiskSettingsPath := filepath.Join(boot.platform.GetDirProvider().BoshDir(), "managed_disk_settings.json")
	var lastMountedCid string

	if boot.platform.GetFs().FileExists(managedDiskSettingsPath) {
		contents, err := boot.platform.GetFs().ReadFile(managedDiskSettingsPath)
		if err != nil {
			return "", bosherr.WrapError(err, "Reading managed_disk_settings.json")
		}
		lastMountedCid = string(contents)

		return lastMountedCid, nil
	}

	return "", nil
}

func (boot bootstrap) mountLastMountedDisk() error {
	lastDiskID, err := boot.lastMountedCid()
	if err != nil {
		return bosherr.WrapError(err, "Fetching last mounted disk CID")
	}

	if lastDiskID == "" {
		return nil
	}

	diskSettings, err := boot.settingsService.GetPersistentDiskSettings(lastDiskID)
	if err != nil {
		return bosherr.WrapError(err, "Fetching disk settings")
	}

	isPartitioned, err := boot.platform.IsPersistentDiskMountable(diskSettings)
	if err != nil {
		return bosherr.WrapError(err, "Checking if persistent disk is partitioned")
	}

	if isPartitioned {
		if err = boot.platform.MountPersistentDisk(diskSettings, boot.dirProvider.StoreDir()); err != nil {
			return bosherr.WrapError(err, "Mounting persistent disk")
		}
	}
	return nil
}
