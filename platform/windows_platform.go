package platform

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	boshcert "github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 -o fakes/fake_windows_disk_manager.go . WindowsDiskManager

type WindowsDiskManager interface {
	GetFormatter() disk.WindowsDiskFormatter
	GetLinker() disk.WindowsDiskLinker
	GetPartitioner() disk.WindowsDiskPartitioner
	GetProtector() disk.WindowsDiskProtector
}

// Administrator user name, this currently exists for testing, but may be useful
// if we ever change the Admin user name for security reasons.
var administratorUserName = "Administrator" //nolint:gochecknoglobals

type WindowsOptions struct {
	// Feature flag during ephemeral disk support rollout
	EnableEphemeralDiskMounting bool
}

type WindowsPlatform struct {
	collector              boshstats.Collector
	fs                     boshsys.FileSystem
	cmdRunner              boshsys.CmdRunner
	compressor             boshcmd.Compressor
	copier                 boshcmd.Copier
	dirProvider            boshdirs.Provider
	vitalsService          boshvitals.Service
	netManager             boshnet.Manager
	devicePathResolver     boshdpresolv.DevicePathResolver
	options                Options
	certManager            boshcert.Manager
	defaultNetworkResolver boshsettings.DefaultNetworkResolver
	auditLogger            AuditLogger
	uuidGenerator          boshuuid.Generator
	diskManager            WindowsDiskManager
	logger                 boshlog.Logger
	logsTarProvider        boshlogstarprovider.LogsTarProvider
}

func NewWindowsPlatform(
	collector boshstats.Collector,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	dirProvider boshdirs.Provider,
	netManager boshnet.Manager,
	certManager boshcert.Manager,
	devicePathResolver boshdpresolv.DevicePathResolver,
	options Options,
	logger boshlog.Logger,
	defaultNetworkResolver boshsettings.DefaultNetworkResolver,
	auditLogger AuditLogger,
	uuidGenerator boshuuid.Generator,
	diskManager WindowsDiskManager,
	logsTarProvider boshlogstarprovider.LogsTarProvider,
) Platform {
	return &WindowsPlatform{
		fs:                     fs,
		cmdRunner:              cmdRunner,
		collector:              collector,
		compressor:             boshcmd.NewTarballCompressor(cmdRunner, fs),
		copier:                 boshcmd.NewGenericCpCopier(fs, logger),
		dirProvider:            dirProvider,
		netManager:             netManager,
		devicePathResolver:     devicePathResolver,
		vitalsService:          boshvitals.NewService(collector, dirProvider, nil),
		certManager:            certManager,
		options:                options,
		defaultNetworkResolver: defaultNetworkResolver,
		auditLogger:            auditLogger,
		uuidGenerator:          uuidGenerator,
		diskManager:            diskManager,
		logger:                 logger,
		logsTarProvider:        logsTarProvider,
	}
}

func (p WindowsPlatform) AssociateDisk(name string, settings boshsettings.DiskSettings) error {
	return errors.New("unimplemented")
}

func (p WindowsPlatform) GetFs() (fs boshsys.FileSystem) {
	return p.fs
}

func (p WindowsPlatform) GetRunner() (runner boshsys.CmdRunner) {
	return p.cmdRunner
}

func (p WindowsPlatform) GetCompressor() (compressor boshcmd.Compressor) {
	return p.compressor
}

func (p WindowsPlatform) GetCopier() (copier boshcmd.Copier) {
	return p.copier
}

func (p WindowsPlatform) GetLogsTarProvider() (logsTarProvider boshlogstarprovider.LogsTarProvider) {
	return p.logsTarProvider
}

func (p WindowsPlatform) GetDirProvider() (dirProvider boshdirs.Provider) {
	return p.dirProvider
}

func (p WindowsPlatform) GetVitalsService() (service boshvitals.Service) {
	return p.vitalsService
}

func (p WindowsPlatform) GetServiceManager() servicemanager.ServiceManager {
	return servicemanager.NewDummyServiceManager()
}

func (p WindowsPlatform) GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver) {
	return p.devicePathResolver
}

func (p WindowsPlatform) GetAuditLogger() AuditLogger {
	return p.auditLogger
}

func (p WindowsPlatform) SetupRuntimeConfiguration() error {
	return setupRuntimeConfiguration()
}

func (p WindowsPlatform) CreateUser(username, _ string) error {
	if err := createUserProfile(username); err != nil {
		return bosherr.WrapError(err, "CreateUser: creating user")
	}
	return nil
}

func (p WindowsPlatform) AddUserToGroups(username string, groups []string) (err error) {
	return
}

func (p WindowsPlatform) findEphemeralUsersMatching(reg *regexp.Regexp) ([]string, error) {
	users, err := localAccountNames()
	if err != nil {
		return nil, bosherr.WrapError(err, "Getting list of users")
	}
	var matchingUsers []string
	for _, user := range users {
		if !strings.HasPrefix(user, boshsettings.EphemeralUserPrefix) {
			continue
		}
		if reg.MatchString(user) {
			matchingUsers = append(matchingUsers, user)
		}
	}
	return matchingUsers, nil
}

const (
	OsErrorFileInUse     = syscall.Errno(0x20)
	OsErrorFileNotFound2 = syscall.Errno(0x2)
	OsErrorFileNotFound3 = syscall.Errno(0x3)
)

func deleteFolderAndContents(path string) error {
	inf, err := statAndIgnoreFileNotFound(path)
	if err != nil {
		return err
	}
	if inf == nil {
		return nil
	}

	err = os.Chmod(path, 0600)
	if err != nil {
		return err
	}

	if inf.IsDir() {
		childItems, _ := os.ReadDir(path) //nolint:errcheck

		for _, item := range childItems {
			err = deleteFolderAndContents(filepath.Join(path, item.Name()))
			if err != nil {
				return err
			}
		}
	}

	return removeAndIgnoreProcessFileLocks(path)
}

func removeAndIgnoreProcessFileLocks(path string) error {
	err := os.RemoveAll(path)
	if err != nil {
		pathError, ok := err.(*os.PathError)
		if !ok {
			fmt.Printf("in RaI after PE")
			return err
		}

		if pathError.Err != OsErrorFileInUse {
			fmt.Printf("after FiU check")
			return err
		}
	}
	return nil
}

func statAndIgnoreFileNotFound(path string) (os.FileInfo, error) {
	inf, err := os.Stat(path)
	if err != nil {
		pathError, ok := err.(*os.PathError)
		if !ok {
			return inf, err
		}

		if pathError.Err != OsErrorFileNotFound2 && pathError.Err != OsErrorFileNotFound3 {
			return inf, err
		}
	}

	return inf, nil
}

func (p WindowsPlatform) DeleteEphemeralUsersMatching(pattern string) error {
	reg, err := regexp.Compile(pattern)
	if err != nil {
		return bosherr.WrapError(err, "Compiling regexp")
	}

	users, err := p.findEphemeralUsersMatching(reg)
	if err != nil {
		return bosherr.WrapError(err, "Finding ephemeral users")
	}

	for _, user := range users {
		err = deleteFolderAndContents(fmt.Sprintf(`C:\Users\%s`, user))
		if err != nil {
			return err
		}

		if err := deleteLocalUser(user); err != nil {
			return err
		}
	}
	return nil
}

func (p WindowsPlatform) SetupRootDisk(ephemeralDiskPath string) (err error) {
	return
}

func (p WindowsPlatform) SetupCanRestartDir() error {
	return nil
}

func (p WindowsPlatform) SetupBoshSettingsDisk() error {
	return nil
}

func (p WindowsPlatform) GetAgentSettingsPath(tmpfs bool) string {
	if tmpfs {
		p.logger.Info("WindowsPlatform", "Windows does not support using tmpfs, using default agent settings path")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "settings.json")
}

func (p WindowsPlatform) GetPersistentDiskSettingsPath(tmpfs bool) string {
	if tmpfs {
		p.logger.Info("WindowsPlatform", "Windows does not support using tmpfs, using default persistent disk settings path")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "persistent_disk_hints.json")
}

func (p WindowsPlatform) GetUpdateSettingsPath(tmpfs bool) string {
	if tmpfs {
		p.logger.Info("WindowsPlatform", "Windows does not support using tmpfs, using default update settings path")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "update_settings.json")
}

func (p WindowsPlatform) SetupSSH(publicKey []string, username string) error {
	if username == boshsettings.VCAPUsername {
		if !userExists(username) {
			err := p.CreateUser(username, "")
			if err != nil {
				return bosherr.WrapErrorf(err, "Creating user: %s", username)
			}
		}
	}

	homedir, err := userHomeDirectory(username)
	if err != nil {
		return bosherr.WrapErrorf(err, "Finding home directory for user: %s", username)
	}

	sshdir := filepath.Join(homedir, ".ssh")
	if err := p.fs.MkdirAll(sshdir, sshDirPermissions); err != nil {
		return bosherr.WrapError(err, "Creating .ssh directory")
	}

	authkeysPath := filepath.Join(sshdir, "authorized_keys")
	publicKeyString := strings.Join(publicKey, "\n")
	if err := p.fs.WriteFileString(authkeysPath, publicKeyString); err != nil {
		return bosherr.WrapErrorf(err, "Creating authorized_keys file: %s", authkeysPath)
	}

	return nil
}

func (p WindowsPlatform) SetUserPassword(user, encryptedPwd string) (err error) {
	if user == boshsettings.VCAPUsername || user == boshsettings.RootUsername {
		//
		// Only randomize the password once.  Otherwise the password will be
		// changed every time the agent restarts - breaking jobs/addons that
		// set the Administrator password.
		//
		if boshnet.LockFileExistsForRandomizedPasswords(p.fs, p.dirProvider) {
			return nil
		}
		if err := setRandomPassword(administratorUserName); err != nil {
			return bosherr.WrapError(err, "Randomized Administrator password")
		}
		if err := boshnet.WriteLockFileForRandomizedPasswords(p.fs, p.dirProvider); err != nil {
			return bosherr.WrapError(err, "Could not set user password")
		}
	}
	return
}

func (p WindowsPlatform) SaveDNSRecords(dnsRecords boshsettings.DNSRecords, hostname string) error {
	windir := os.Getenv("windir")
	if windir == "" {
		return bosherr.Error("SaveDNSRecords: missing %WINDIR% env variable")
	}

	etcdir := filepath.Join(windir, "System32", "Drivers", "etc")
	if err := p.fs.MkdirAll(etcdir, 0755); err != nil {
		return bosherr.WrapError(err, "SaveDNSRecords: creating etc directory")
	}

	uuid, err := p.uuidGenerator.Generate()
	if err != nil {
		return bosherr.WrapError(err, "SaveDNSRecords: generating UUID")
	}

	tmpfile := filepath.Join(etcdir, "hosts-"+uuid)
	f, err := p.fs.OpenFile(tmpfile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
	if err != nil {
		return bosherr.WrapError(err, "SaveDNSRecords: opening hosts file")
	}

	var buf bytes.Buffer
	for _, rec := range dnsRecords.Records {
		fmt.Fprintf(&buf, "%s %s\n", rec[0], rec[1])
	}
	if _, err := buf.WriteTo(f); err != nil {
		f.Close() //nolint:errcheck
		return bosherr.WrapErrorf(err, "SaveDNSRecords: writing DNS records to: %s", tmpfile)
	}
	// Explicitly close before renaming - required to release handle
	f.Close() //nolint:errcheck

	hostfile := filepath.Join(etcdir, "hosts")
	if err := p.fs.Rename(tmpfile, hostfile); err != nil {
		return bosherr.WrapErrorf(err, "SaveDNSRecords: renaming %s to %s", tmpfile, hostfile)
	}
	return nil
}

func (p WindowsPlatform) SetupIPv6(config boshsettings.IPv6) error {
	return nil
}

func (p WindowsPlatform) SetupHostname(hostname string) (err error) {
	return
}

func (p WindowsPlatform) SetupNetworking(networks boshsettings.Networks, mbus string) (err error) {
	return p.netManager.SetupNetworking(networks, mbus, nil)
}

func (p WindowsPlatform) GetConfiguredNetworkInterfaces() (interfaces []string, err error) {
	return p.netManager.GetConfiguredNetworkInterfaces()
}

func (p WindowsPlatform) GetCertManager() (certManager boshcert.Manager) {
	return p.certManager
}

func (p WindowsPlatform) SetupLogrotate(groupName, basePath, size string) error {
	return nil
}

func (p WindowsPlatform) SetTimeWithNtpServers(servers []string) error {
	if len(servers) == 0 {
		return nil
	}

	ntpServers := strings.Join(servers, " ")
	_, stderr, _, err := p.cmdRunner.RunCommand("powershell.exe",
		"new-netfirewallrule",
		"-displayname", "NTP",
		"-direction", "outbound",
		"-action", "allow",
		"-protocol", "udp",
		"-RemotePort", "123")
	if err != nil {
		return bosherr.WrapErrorf(err, "SetTimeWithNtpServers  %s", stderr)
	}

	_, _, _, _ = p.cmdRunner.RunCommand("net", "stop", "w32time") //nolint:errcheck
	manualPeerList := fmt.Sprintf("/manualpeerlist:\"%s\"", ntpServers)
	_, stderr, _, err = p.cmdRunner.RunCommand(
		"powershell.exe",
		"w32tm",
		"/config",
		"/syncfromflags:manual",
		manualPeerList)
	if err != nil {
		return bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
	}
	_, _, _, _ = p.cmdRunner.RunCommand("net", "start", "w32time") //nolint:errcheck
	_, stderr, _, err = p.cmdRunner.RunCommand("w32tm", "/config", "/update")
	if err != nil {
		return bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
	}
	_, stderr, _, err = p.cmdRunner.RunCommand("w32tm", "/resync", "/rediscover")
	if err != nil {
		return bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
	}
	return nil
}

func (p WindowsPlatform) SetupEphemeralDiskWithPath(devicePath string, desiredSwapSizeInBytes *uint64, labelPrefix string) error {
	const minimumDiskSizeToPartition = 1024 * 1024

	if devicePath == "" || !p.options.Windows.EnableEphemeralDiskMounting {
		p.logger.Debug("WindowsPlatform", "Not attempting to mount ephemeral disk with devicePath `%s`", devicePath)
		return nil
	}

	dataPath := fmt.Sprintf(`C:%s\`, p.dirProvider.DataDir())

	protector := p.diskManager.GetProtector()
	if !protector.CommandExists() {
		return fmt.Errorf("cannot protect %s. %s cmd does not exist", dataPath, disk.ProtectCmdlet)
	}

	partitioner := p.diskManager.GetPartitioner()

	if devicePath != "0" {
		existingPartitionCount, err := partitioner.GetCountOnDisk(devicePath)
		if err != nil {
			return err
		}

		if existingPartitionCount == "0" {
			err = partitioner.InitializeDisk(devicePath)
			if err != nil {
				return err
			}
		}
	}

	linker := p.diskManager.GetLinker()

	existingTarget, err := linker.LinkTarget(dataPath)
	if err != nil {
		return err
	}

	if existingTarget != "" {
		return nil
	}

	freeSpace, err := partitioner.GetFreeSpaceOnDisk(devicePath)
	if err != nil {
		return err
	}

	if freeSpace < minimumDiskSizeToPartition {
		p.logger.Warn(
			"WindowsPlatform",
			"Unable to create ephemeral partition on disk %s, as there isn't enough free space",
			devicePath,
		)
		return nil
	}

	partitionNumber, err := partitioner.PartitionDisk(devicePath)
	if err != nil {
		return err
	}

	formatter := p.diskManager.GetFormatter()

	err = formatter.Format(devicePath, partitionNumber)
	if err != nil {
		return err
	}

	driveLetter, err := partitioner.AssignDriveLetter(devicePath, partitionNumber)
	if err != nil {
		return err
	}

	err = linker.Link(dataPath, fmt.Sprintf("%s:", driveLetter))
	if err != nil {
		return err
	}

	err = protector.ProtectPath(dataPath)
	if err != nil {
		return err
	}

	return nil
}

func (p WindowsPlatform) SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error) {
	return
}

func (p WindowsPlatform) SetupDataDir(_ boshsettings.JobDir, _ boshsettings.RunDir) error {
	dataDir := p.dirProvider.DataDir()
	sysDataDir := filepath.Join(dataDir, "sys")
	logDir := filepath.Join(sysDataDir, "log")

	if err := p.fs.MkdirAll(logDir, logDirPermissions); err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", logDir)
	}

	sysDir := filepath.Join(p.dirProvider.BaseDir(), "sys")

	if !p.fs.FileExists(sysDir) {
		if err := p.fs.Symlink(sysDataDir, sysDir); err != nil {
			return bosherr.WrapErrorf(err, "Symlinking '%s' to '%s'", sysDir, sysDataDir)
		}
	}
	return nil
}

func (p WindowsPlatform) SetupHomeDir() error {
	return nil
}

func (p WindowsPlatform) SetupSharedMemory() error {
	return nil
}

func (p WindowsPlatform) SetupTmpDir() error {
	boshTmpDir := p.dirProvider.TmpDir()

	err := p.fs.MkdirAll(boshTmpDir, tmpDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating temp dir")
	}

	systemTemp := os.TempDir()
	err = p.fs.Symlink(boshTmpDir, systemTemp)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Creating symlink from %s to %s", systemTemp, boshTmpDir))
	}

	err = os.Setenv("TMP", boshTmpDir)
	if err != nil {
		return bosherr.WrapError(err, "Setting TMP")
	}

	err = os.Setenv("TEMP", boshTmpDir)
	if err != nil {
		return bosherr.WrapError(err, "Setting TEMP")
	}

	return nil
}

func (p WindowsPlatform) SetupLogDir() error {
	return nil
}

func (p WindowsPlatform) SetupOptDir() error {
	return nil
}

func (p WindowsPlatform) SetupBlobsDir() error {
	blobsDirPath := p.dirProvider.BlobsDir()
	err := p.fs.MkdirAll(blobsDirPath, blobsDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating blobs dir")
	}
	return nil
}

func (p WindowsPlatform) SetupLoggingAndAuditing() error {
	return nil
}

func (p WindowsPlatform) AdjustPersistentDiskPartitioning(diskSettings boshsettings.DiskSettings, mountPoint string) (err error) {
	return
}

func (p WindowsPlatform) MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) (err error) {
	return
}

func (p WindowsPlatform) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (didUnmount bool, err error) {
	return
}

func (p WindowsPlatform) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) (diskPath string, err error) {
	p.logger.Debug("WindowsPlatform", "Identifying ephemeral disk path, diskSettings.Path: `%s`", diskSettings.Path)

	if p.options.Linux.CreatePartitionIfNoEphemeralDisk {
		diskPath = "0"
	}

	if diskSettings.Path != "" { //nolint:nestif
		matchInt, _ := regexp.MatchString(`\d`, diskSettings.Path) //nolint:errcheck
		if matchInt {
			diskPath = diskSettings.Path
		} else {
			alphs := []byte("abcdefghijklmnopq")

			lastChar := diskSettings.Path[len(diskSettings.Path)-1:]
			diskPath = fmt.Sprintf("%d", bytes.IndexByte(alphs, lastChar[0]))
		}
	} else if diskSettings.DeviceID != "" {
		stdout, stderr, _, err := p.cmdRunner.RunCommand("powershell", "-Command", fmt.Sprintf("Get-Disk -UniqueId %s | Select Number | ConvertTo-Json", strings.ReplaceAll(diskSettings.DeviceID, "-", "")))
		if err != nil {
			return "", bosherr.WrapErrorf(err, "Translating disk ID to disk number: %s: %s", err.Error(), stderr)
		}
		var diskNumberResponse struct {
			Number json.Number
		}
		err = json.Unmarshal([]byte(stdout), &diskNumberResponse)
		if err != nil {
			return "", bosherr.WrapError(err, "Translating disk ID to disk number")
		}
		diskPath = string(diskNumberResponse.Number)
	}

	p.logger.Debug("WindowsPlatform", "Identified Disk Path as `%s`", diskPath)

	return diskPath, nil
}

func (p WindowsPlatform) GetFileContentsFromCDROM(filePath string) (contents []byte, err error) {
	return p.fs.ReadFile("D:/" + filePath)
}

func (p WindowsPlatform) GetFilesContentsFromDisk(diskPath string, fileNames []string) (contents [][]byte, err error) {
	return
}

func (p WindowsPlatform) MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error) {
	return
}

func (p WindowsPlatform) IsMountPoint(path string) (string, bool, error) {
	return "", true, nil
}

func (p WindowsPlatform) IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (bool, error) {
	return true, nil
}

func (p WindowsPlatform) IsPersistentDiskMountable(diskSettings boshsettings.DiskSettings) (bool, error) {
	return true, nil
}

func (p WindowsPlatform) StartMonit() (err error) {
	return
}

func (p WindowsPlatform) SetupMonitUser() (err error) {
	return
}

func (p WindowsPlatform) GetMonitCredentials() (username, password string, err error) {
	return
}

func (p WindowsPlatform) PrepareForNetworkingChange() error {
	return nil
}

func (p WindowsPlatform) CleanIPMacAddressCache(ip string) error {
	return nil
}

func (p WindowsPlatform) RemoveDevTools(packageFileListPath string) error {
	return nil
}

func (p WindowsPlatform) RemoveStaticLibraries(packageFileListPath string) error {
	return nil
}

func (p WindowsPlatform) GetDefaultNetwork(ipProtocol boship.IPProtocol) (boshsettings.Network, error) {
	return p.defaultNetworkResolver.GetDefaultNetwork(ipProtocol)
}

func (p WindowsPlatform) GetHostPublicKey() (string, error) {
	if err := sshEnabled(); err != nil {
		return "", bosherr.WrapError(err, "OpenSSH is not running")
	}

	drive := os.Getenv("SYSTEMDRIVE")
	if drive == "" {
		drive = "C:"
	}
	drive += "\\"

	sshdir := filepath.Join(drive, "ProgramData", "ssh")
	keypath := filepath.Join(sshdir, "ssh_host_rsa_key.pub")

	key, err := p.fs.ReadFileString(keypath)
	if err != nil {
		// Provide a useful error message.
		//
		// Do this here otherwise the FakeFileSystem we use for tests
		// incorrectly complains that the directories we created don't
		// exist.
		//
		if _, err := p.fs.Stat(sshdir); os.IsNotExist(err) {
			return "", bosherr.WrapErrorf(err, "Reading host public key: "+
				"expected OpenSSH to be installed at: %s", sshdir)
		}
		return "", bosherr.WrapErrorf(err, "Missing host public RSA key: %s", keypath)
	}
	return key, nil
}

func (p WindowsPlatform) DeleteARPEntryWithIP(ip string) error {
	_, _, _, err := p.cmdRunner.RunCommand("arp", "-d", ip)
	if err != nil {
		return bosherr.WrapError(err, "Deleting arp entry")
	}

	return nil
}

func (p WindowsPlatform) SetupRecordsJSONPermission(path string) error {
	return nil
}

func (p WindowsPlatform) SetupFirewall() error {
	return nil
}

func (p WindowsPlatform) Shutdown() error {
	return nil
}

func (p WindowsPlatform) GetNatsFirewallHook() firewall.NatsFirewallHook {
	return nil
}
