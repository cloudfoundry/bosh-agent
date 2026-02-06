package platform

import (
	"log"

	"github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"

	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

type AuditLogger interface {
	Debug(string)
	Err(string)
	StartLogging()
}

type AuditLoggerProvider interface {
	ProvideDebugLogger() (*log.Logger, error)
	ProvideErrorLogger() (*log.Logger, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -generate
//counterfeiter:generate . Platform
//counterfeiter:generate . AuditLogger

type Platform interface {
	GetFs() boshsys.FileSystem
	GetRunner() boshsys.CmdRunner
	GetCompressor() boshcmd.Compressor
	GetCopier() boshcmd.Copier
	GetDirProvider() boshdir.Provider
	GetVitalsService() boshvitals.Service
	GetAuditLogger() AuditLogger
	GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver)
	GetServiceManager() servicemanager.ServiceManager
	GetAgentSettingsPath(tmpfs bool) string
	GetPersistentDiskSettingsPath(tmpfs bool) string
	GetUpdateSettingsPath(tmpfs bool) string
	GetLogsTarProvider() boshlogstarprovider.LogsTarProvider

	// User management
	CreateUser(username, basePath string) (err error)
	AddUserToGroups(username string, groups []string) (err error)
	DeleteEphemeralUsersMatching(regex string) (err error)

	// Bootstrap functionality
	SetupRootDisk(ephemeralDiskPath string) (err error)
	SetupSSH(publicKey []string, username string) (err error)
	SetUserPassword(user, encryptedPwd string) (err error)
	SetupBoshSettingsDisk() (err error)
	SetupIPv6(boshsettings.IPv6) error
	SetupHostname(hostname string) (err error)
	SetupNetworking(networks boshsettings.Networks, mbus string) (err error)
	SetupLogrotate(groupName, basePath, size string) (err error)
	SetTimeWithNtpServers(servers []string) (err error)
	SetupEphemeralDiskWithPath(devicePath string, desiredSwapSizeInBytes *uint64, labelPrefix string) (err error)
	SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error)
	SetupDataDir(boshsettings.JobDir, boshsettings.RunDir) (err error)
	SetupSharedMemory() (err error)
	SetupTmpDir() (err error)
	SetupCanRestartDir() (err error)
	SetupHomeDir() (err error)
	SetupBlobsDir() (err error)
	SetupMonitUser() (err error)
	StartMonit() (err error)
	SetupRuntimeConfiguration() (err error)
	SetupLogDir() (err error)
	SetupLoggingAndAuditing() (err error)
	SetupOptDir() (err error)
	SetupRecordsJSONPermission(path string) error
	SetupFirewall() error

	// Disk management
	AdjustPersistentDiskPartitioning(diskSettings boshsettings.DiskSettings, mountPoint string) error
	MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) error
	UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (didUnmount bool, err error)
	MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error)
	GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) (string, error)
	IsMountPoint(path string) (partitionPath string, result bool, err error)
	IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (result bool, err error)
	IsPersistentDiskMountable(diskSettings boshsettings.DiskSettings) (bool, error)
	AssociateDisk(name string, settings boshsettings.DiskSettings) error

	GetFileContentsFromCDROM(filePath string) (contents []byte, err error)
	GetFilesContentsFromDisk(diskPath string, fileNames []string) (contents [][]byte, err error)

	// Network misc
	GetDefaultNetwork(ipProtocol boship.IPProtocol) (boshsettings.Network, error)
	GetConfiguredNetworkInterfaces() ([]string, error)
	PrepareForNetworkingChange() error
	DeleteARPEntryWithIP(ip string) error
	SaveDNSRecords(dnsRecords boshsettings.DNSRecords, hostname string) error

	// Additional monit management
	GetMonitCredentials() (username, password string, err error)

	GetCertManager() cert.Manager

	GetHostPublicKey() (string, error)

	RemoveDevTools(packageFileListPath string) error
	RemoveStaticLibraries(packageFileListPath string) error

	Shutdown() error

	// Firewall management
	// GetNatsFirewallHook returns a hook that is called before NATS connection/reconnection
	// to update firewall rules with resolved DNS. Returns nil if firewall is not supported.
	GetNatsFirewallHook() firewall.NatsFirewallHook
}
