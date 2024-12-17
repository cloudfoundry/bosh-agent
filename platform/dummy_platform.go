package platform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	boshcert "github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

type mount struct {
	MountDir     string
	MountOptions []string
	DiskCid      string
}

type formattedDisk struct {
	DiskCid string
}

type diskMigration struct {
	FromDiskCid string
	ToDiskCid   string
}

const CredentialFileName = "password"
const EtcHostsFileName = "etc_hosts"

type dummyPlatform struct {
	collector          boshstats.Collector
	fs                 boshsys.FileSystem
	cmdRunner          boshsys.CmdRunner
	compressor         boshcmd.Compressor
	copier             boshcmd.Copier
	dirProvider        boshdirs.Provider
	vitalsService      boshvitals.Service
	devicePathResolver boshdpresolv.DevicePathResolver
	logger             boshlog.Logger
	certManager        boshcert.Manager
	auditLogger        AuditLogger
	logsTarProvider    boshlogstarprovider.LogsTarProvider
}

func NewDummyPlatform(
	collector boshstats.Collector,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	dirProvider boshdirs.Provider,
	devicePathResolver boshdpresolv.DevicePathResolver,
	logger boshlog.Logger,
	auditLogger AuditLogger,
	logsTarProvider boshlogstarprovider.LogsTarProvider,
) Platform {
	return &dummyPlatform{
		fs:                 fs,
		cmdRunner:          cmdRunner,
		collector:          collector,
		compressor:         boshcmd.NewTarballCompressor(cmdRunner, fs),
		copier:             boshcmd.NewGenericCpCopier(fs, logger),
		dirProvider:        dirProvider,
		devicePathResolver: devicePathResolver,
		vitalsService:      boshvitals.NewService(collector, dirProvider, nil),
		certManager:        boshcert.NewDummyCertManager(fs, cmdRunner, 0, logger),
		logger:             logger,
		auditLogger:        auditLogger,
		logsTarProvider:    logsTarProvider,
	}
}

func (p dummyPlatform) GetFs() (fs boshsys.FileSystem) {
	return p.fs
}

func (p dummyPlatform) GetAuditLogger() AuditLogger {
	return p.auditLogger
}

func (p dummyPlatform) GetRunner() (runner boshsys.CmdRunner) {
	return p.cmdRunner
}

func (p dummyPlatform) GetCompressor() (compressor boshcmd.Compressor) {
	return p.compressor
}

func (p dummyPlatform) GetCopier() (copier boshcmd.Copier) {
	return p.copier
}

func (p dummyPlatform) GetLogsTarProvider() (logsTarProvider boshlogstarprovider.LogsTarProvider) {
	return p.logsTarProvider
}

func (p dummyPlatform) GetDirProvider() (dirProvider boshdirs.Provider) {
	return p.dirProvider
}

func (p dummyPlatform) GetVitalsService() (service boshvitals.Service) {
	return p.vitalsService
}

func (p dummyPlatform) GetServiceManager() servicemanager.ServiceManager {
	return servicemanager.NewDummyServiceManager()
}

func (p dummyPlatform) GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver) {
	return p.devicePathResolver
}

func (p dummyPlatform) SetupRuntimeConfiguration() (err error) {
	return
}

func (p dummyPlatform) CreateUser(username, basePath string) (err error) {
	return
}

func (p dummyPlatform) AddUserToGroups(username string, groups []string) (err error) {
	return
}

func (p dummyPlatform) DeleteEphemeralUsersMatching(regex string) (err error) {
	return
}

func (p dummyPlatform) SetupRootDisk(ephemeralDiskPath string) (err error) {
	return
}

func (p dummyPlatform) SetupSSH(publicKey []string, username string) (err error) {
	return
}

func (p dummyPlatform) SetUserPassword(user, encryptedPwd string) (err error) {
	credentialsPath := filepath.Join(p.dirProvider.BoshDir(), user, CredentialFileName)
	return p.fs.WriteFileString(credentialsPath, encryptedPwd)
}

func (p dummyPlatform) SaveDNSRecords(dnsRecords boshsettings.DNSRecords, hostname string) (err error) {
	etcHostsPath := filepath.Join(p.dirProvider.BoshDir(), EtcHostsFileName)

	dnsRecordsContents := bytes.NewBuffer([]byte{})
	for _, dnsRecord := range dnsRecords.Records {
		dnsRecordsContents.WriteString(fmt.Sprintf("%s %s\n", dnsRecord[0], dnsRecord[1]))
	}

	return p.fs.WriteFileString(etcHostsPath, dnsRecordsContents.String())
}

func (p dummyPlatform) SetupBoshSettingsDisk() error {
	return p.fs.MkdirAll(filepath.Dir(p.GetAgentSettingsPath(true)), 0700)
}

func (p dummyPlatform) GetAgentSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "settings.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "settings.json")
}

func (p dummyPlatform) GetPersistentDiskSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "persistent_disk_hints.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "persistent_disk_hints.json")
}

func (p dummyPlatform) GetUpdateSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "update_settings.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "update_settings.json")
}

func (p dummyPlatform) SetupIPv6(config boshsettings.IPv6) error {
	return nil
}

func (p dummyPlatform) SetupHostname(hostname string) (err error) {
	return
}

func (p dummyPlatform) SetupNetworking(networks boshsettings.Networks, mbus string) (err error) {
	return
}
func (p dummyPlatform) GetConfiguredNetworkInterfaces() (interfaces []string, err error) {
	return
}

func (p dummyPlatform) GetCertManager() (certManager boshcert.Manager) {
	return p.certManager
}

func (p dummyPlatform) SetupLogrotate(groupName, basePath, size string) (err error) {
	return
}

func (p dummyPlatform) SetTimeWithNtpServers(servers []string) (err error) {
	return
}

func (p dummyPlatform) SetupEphemeralDiskWithPath(devicePath string, desiredSwapSizeInBytes *uint64, labelPrefix string) (err error) {
	return
}

func (p dummyPlatform) SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error) {
	return
}

func (p dummyPlatform) SetupDataDir(_ boshsettings.JobDir, _ boshsettings.RunDir) error {
	dataDir := p.dirProvider.DataDir()

	sysDataDir := filepath.Join(dataDir, "sys")

	logDir := filepath.Join(sysDataDir, "log")
	err := p.fs.MkdirAll(logDir, logDirPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", logDir)
	}

	sysDir := filepath.Join(filepath.Dir(dataDir), "sys")
	err = p.fs.Symlink(sysDataDir, sysDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Symlinking '%s' to '%s'", sysDir, sysDataDir)
	}

	return nil
}

func (p dummyPlatform) SetupCanRestartDir() error {
	return nil
}

func (p dummyPlatform) SetupTmpDir() error {
	return nil
}

func (p dummyPlatform) SetupHomeDir() error {
	return nil
}

func (p dummyPlatform) SetupLogDir() error {
	return nil
}

func (p dummyPlatform) SetupOptDir() error {
	return nil
}

func (p dummyPlatform) SetupSharedMemory() error {
	return nil
}

func (p dummyPlatform) SetupBlobsDir() error {
	blobsDir := p.dirProvider.BlobsDir()
	if err := p.fs.MkdirAll(blobsDir, blobsDirPermissions); err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", blobsDir)
	}

	return nil
}

func (p dummyPlatform) SetupLoggingAndAuditing() error {
	return nil
}

func (p dummyPlatform) AdjustPersistentDiskPartitioning(diskSettings boshsettings.DiskSettings, mountPoint string) error {
	return nil
}

func (p dummyPlatform) MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error {
	mounts, err := p.existingMounts()
	if err != nil {
		return err
	}

	_, isMountPoint, err := p.IsMountPoint(mountPoint)
	if err != nil {
		return err
	}

	managedSettingsPath := filepath.Join(p.dirProvider.BoshDir(), "managed_disk_settings.json")

	if isMountPoint {
		currentManagedDisk, err := p.fs.ReadFileString(managedSettingsPath)
		if err != nil {
			return err
		}

		if diskSettings.ID == currentManagedDisk {
			return nil
		}

		mountPoint = p.dirProvider.StoreMigrationDir()
	}

	newlyFormattedDisk := []formattedDisk{{DiskCid: diskSettings.ID}}
	diskJSON, err := json.Marshal(newlyFormattedDisk)
	if err != nil {
		return err
	}

	err = p.fs.WriteFile(filepath.Join(p.dirProvider.BoshDir(), "formatted_disks.json"), diskJSON)
	if err != nil {
		return err
	}

	mounts = append(mounts, mount{
		MountDir:     mountPoint,
		MountOptions: diskSettings.MountOptions,
		DiskCid:      diskSettings.ID,
	})
	mountsJSON, err := json.Marshal(mounts)
	if err != nil {
		return err
	}

	err = p.fs.WriteFileString(managedSettingsPath, diskSettings.ID)
	if err != nil {
		return err
	}

	return p.fs.WriteFile(p.mountsPath(), mountsJSON)
}

func (p dummyPlatform) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (didUnmount bool, err error) {
	mounts, err := p.existingMounts()
	if err != nil {
		return false, err
	}

	var updatedMounts []mount
	for _, mount := range mounts {
		if mount.DiskCid != diskSettings.ID {
			updatedMounts = append(updatedMounts, mount)
		}
	}

	updatedMountsJSON, err := json.Marshal(updatedMounts)
	if err != nil {
		return false, err
	}

	err = p.fs.WriteFile(p.mountsPath(), updatedMountsJSON)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (p dummyPlatform) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) (string, error) {
	return "/dev/sdb", nil
}

func (p dummyPlatform) GetFileContentsFromCDROM(filePath string) (contents []byte, err error) {
	return
}

func (p dummyPlatform) GetFilesContentsFromDisk(diskPath string, fileNames []string) (contents [][]byte, err error) {
	return
}

func (p dummyPlatform) MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error) {
	diskMigrationsPath := filepath.Join(p.dirProvider.BoshDir(), "disk_migrations.json")
	var diskMigrations []diskMigration
	if p.fs.FileExists(diskMigrationsPath) {
		bytes, err := p.fs.ReadFile(diskMigrationsPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(bytes, &diskMigrations)
		if err != nil {
			return err
		}
	}

	mounts, err := p.existingMounts()
	if err != nil {
		return err
	}
	fromDiskCid := p.getDiskCidByMountPoint(fromMountPoint, mounts)
	toDiskCid := p.getDiskCidByMountPoint(toMountPoint, mounts)

	diskMigrations = append(diskMigrations, diskMigration{FromDiskCid: fromDiskCid, ToDiskCid: toDiskCid})

	diskMigrationsJSON, err := json.Marshal(diskMigrations)
	if err != nil {
		return err
	}

	return p.fs.WriteFile(diskMigrationsPath, diskMigrationsJSON)
}

func (p dummyPlatform) IsMountPoint(mountPointPath string) (partitionPath string, result bool, err error) {
	mounts, err := p.existingMounts()
	if err != nil {
		return "", false, err
	}

	for _, mount := range mounts {
		if mount.MountDir == mountPointPath {
			return "", true, nil
		}
	}

	return "", false, nil
}

func (p dummyPlatform) IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (bool, error) {
	return true, nil
}

func (p dummyPlatform) IsPersistentDiskMountable(diskSettings boshsettings.DiskSettings) (bool, error) {
	var formattedDisks []formattedDisk
	formattedDisksPath := filepath.Join(p.dirProvider.BoshDir(), "formatted_disks.json")
	if p.fs.FileExists(formattedDisksPath) {
		bytes, err := p.fs.ReadFile(formattedDisksPath)
		if err != nil {
			return false, err
		}
		err = json.Unmarshal(bytes, &formattedDisks)
		if err != nil {
			return false, err
		}

		for _, disk := range formattedDisks {
			if diskSettings.ID == disk.DiskCid {
				return true, nil
			}
		}
	}

	return false, nil
}

func (p dummyPlatform) AssociateDisk(name string, settings boshsettings.DiskSettings) error {
	diskAssocsPath := filepath.Join(p.dirProvider.BoshDir(), "disk_associations.json")

	diskNames := []string{}

	bytes, err := p.fs.ReadFile(diskAssocsPath)
	if err != nil {
		err, ok := err.(bosherr.ComplexError)
		if !ok {
			return bosherr.WrapError(err, "Associating Disk: ")
		} else if !os.IsNotExist(err.Cause) {
			return bosherr.WrapError(err, "Associating Disk: ")
		}
	} else if err == nil {
		err = json.Unmarshal(bytes, &diskNames)
		if err != nil {
			return err
		}
	}

	diskNames = append(diskNames, name)

	contents, err := json.Marshal(diskNames)
	if err != nil {
		return err
	}

	return p.fs.WriteFile(diskAssocsPath, contents)
}

func (p dummyPlatform) StartMonit() (err error) {
	return
}

func (p dummyPlatform) SetupMonitUser() (err error) {
	return
}

func (p dummyPlatform) GetMonitCredentials() (username, password string, err error) {
	return
}

func (p dummyPlatform) PrepareForNetworkingChange() error {
	return nil
}

func (p dummyPlatform) DeleteARPEntryWithIP(ip string) error {
	return nil
}

func (p dummyPlatform) GetDefaultNetwork() (boshsettings.Network, error) {
	var network boshsettings.Network

	networkPath := filepath.Join(p.dirProvider.BoshDir(), "dummy-default-network-settings.json")
	contents, err := p.fs.ReadFile(networkPath)
	if err != nil {
		return network, nil
	}

	err = json.Unmarshal(contents, &network)
	if err != nil {
		return network, bosherr.WrapError(err, "Unmarshal json settings")
	}

	return network, nil
}

func (p dummyPlatform) GetHostPublicKey() (string, error) {
	return "dummy-public-key", nil
}

func (p dummyPlatform) RemoveDevTools(packageFileListPath string) error {
	return nil
}

func (p dummyPlatform) RemoveStaticLibraries(packageFileListPath string) error {
	return nil
}

func (p dummyPlatform) getDiskCidByMountPoint(mountPoint string, mounts []mount) string {
	var diskCid string
	for _, mount := range mounts {
		if mount.MountDir == mountPoint {
			diskCid = mount.DiskCid
		}
	}
	return diskCid
}

func (p dummyPlatform) mountsPath() string {
	return filepath.Join(p.dirProvider.BoshDir(), "mounts.json")
}

func (p dummyPlatform) existingMounts() ([]mount, error) {
	mountsPath := p.mountsPath()
	var mounts []mount

	if !p.fs.FileExists(mountsPath) {
		return mounts, nil
	}

	bytes, err := p.fs.ReadFile(mountsPath)
	if err != nil {
		return mounts, err
	}
	err = json.Unmarshal(bytes, &mounts)
	return mounts, err
}

func (p dummyPlatform) SetupRecordsJSONPermission(path string) error {
	return nil
}

func (p dummyPlatform) Shutdown() error {
	return nil
}
