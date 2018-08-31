package platform

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshcert "github.com/cloudfoundry/bosh-agent/platform/cert"
	boshnet "github.com/cloudfoundry/bosh-agent/platform/net"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

//go:generate counterfeiter -o fakes/fake_windows_disk_formatter.go . WindowsDiskFormatter

type WindowsDiskFormatter interface {
	Format(diskNumber, partitionNumber string) error
}

//go:generate counterfeiter -o fakes/fake_windows_disk_linker.go . WindowsDiskLinker

type WindowsDiskLinker interface {
	IsLinked(location string) (target string, err error)
}

// Administrator user name, this currently exists for testing, but may be useful
// if we ever change the Admin user name for security reasons.
var administratorUserName = "Administrator"

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
	formatter              WindowsDiskFormatter
	linker                 WindowsDiskLinker
	logger                 boshlog.Logger
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
	formatter WindowsDiskFormatter,
	linker WindowsDiskLinker,
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
		vitalsService:          boshvitals.NewService(collector, dirProvider),
		certManager:            certManager,
		options:                options,
		defaultNetworkResolver: defaultNetworkResolver,
		auditLogger:            auditLogger,
		uuidGenerator:          uuidGenerator,
		formatter:              formatter,
		linker:                 linker,
		logger:                 logger,
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

func (p WindowsPlatform) GetDirProvider() (dirProvider boshdir.Provider) {
	return p.dirProvider
}

func (p WindowsPlatform) GetVitalsService() (service boshvitals.Service) {
	return p.vitalsService
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
		if err := deleteUserProfile(user); err != nil {
			return err
		}
	}
	return nil
}

func (p WindowsPlatform) SetupRootDisk(ephemeralDiskPath string) (err error) {
	return
}

func (p WindowsPlatform) SetupSSH(publicKey []string, username string) error {

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

func (p WindowsPlatform) SaveDNSRecords(dnsRecords boshsettings.DNSRecords, hostname string) (err error) {
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
		f.Close()
		return bosherr.WrapErrorf(err, "SaveDNSRecords: writing DNS records to: %s", tmpfile)
	}
	f.Close() // Explicitly close before renaming - required to release handle

	hostfile := filepath.Join(etcdir, "hosts")
	if err := p.fs.Rename(tmpfile, hostfile); err != nil {
		return bosherr.WrapErrorf(err, "SaveDNSRecords: renaming %s to %s", tmpfile, hostfile)
	}
	return
}

func (p WindowsPlatform) SetupIPv6(config boshsettings.IPv6) error {
	return nil
}

func (p WindowsPlatform) SetupHostname(hostname string) (err error) {
	return
}

func (p WindowsPlatform) SetupNetworking(networks boshsettings.Networks) (err error) {
	return p.netManager.SetupNetworking(networks, nil)
}

func (p WindowsPlatform) GetConfiguredNetworkInterfaces() (interfaces []string, err error) {
	return p.netManager.GetConfiguredNetworkInterfaces()
}

func (p WindowsPlatform) GetCertManager() (certManager boshcert.Manager) {
	return p.certManager
}

func (p WindowsPlatform) SetupLogrotate(groupName, basePath, size string) (err error) {
	return
}

func (p WindowsPlatform) SetTimeWithNtpServers(servers []string) (err error) {
	if len(servers) == 0 {
		return
	}
	var (
		stderr string
	)
	ntpServers := strings.Join(servers, " ")
	_, stderr, _, err = p.cmdRunner.RunCommand("powershell.exe",
		"new-netfirewallrule",
		"-displayname", "NTP",
		"-direction", "outbound",
		"-action", "allow",
		"-protocol", "udp",
		"-RemotePort", "123")
	if err != nil {
		err = bosherr.WrapErrorf(err, "SetTimeWithNtpServers  %s", stderr)
		return
	}

	_, _, _, _ = p.cmdRunner.RunCommand("net", "stop", "w32time")
	manualPeerList := fmt.Sprintf("/manualpeerlist:\"%s\"", ntpServers)
	_, stderr, _, err = p.cmdRunner.RunCommand(
		"powershell.exe",
		"w32tm",
		"/config",
		"/syncfromflags:manual",
		manualPeerList)
	if err != nil {
		err = bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
		return
	}
	_, _, _, _ = p.cmdRunner.RunCommand("net", "start", "w32time")
	_, stderr, _, err = p.cmdRunner.RunCommand("w32tm", "/config", "/update")
	if err != nil {
		err = bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
		return
	}
	_, stderr, _, err = p.cmdRunner.RunCommand("w32tm", "/resync", "/rediscover")
	if err != nil {
		err = bosherr.WrapErrorf(err, "SetTimeWithNtpServers %s", stderr)
		return
	}
	return
}

func (p WindowsPlatform) SetupEphemeralDiskWithPath(devicePath string, desiredSwapSizeInBytes *uint64) error {
	if devicePath == "" || !p.options.Windows.EnableEphemeralDiskMounting {
		p.logger.Debug("WindowsPlatform", "Not attempting to mount ephemeral disk with devicePath `%s`", devicePath)
		return nil
	}

	dataPath := fmt.Sprintf(`C:%s\`, p.dirProvider.DataDir())

	checkProtectPathCmdlet := &powershellAction{
		commandArgs: []string{
			"Get-Command",
			"Protect-Path",
		},
		commandFailureFmt: fmt.Sprintf("Cannot protect %s. Protect-Path cmd does not exist: %%s", dataPath),
		cmdRunner:         p.cmdRunner,
	}

	_, err := checkProtectPathCmdlet.run()

	if err != nil {
		return err
	}

	if devicePath != "0" {
		getExistingPartitionCountAction := &powershellAction{
			commandArgs: []string{
				"Get-Disk",
				"-Number",
				devicePath,
				"|",
				"Select",
				"-ExpandProperty",
				"NumberOfPartitions",
			},
			commandFailureFmt: fmt.Sprintf("Failed to get existing partition count for disk %s: %%s", devicePath),
			cmdRunner:         p.cmdRunner,
		}

		stdout, err := getExistingPartitionCountAction.run()

		if err != nil {
			return err
		}

		if strings.TrimSpace(stdout) == "0" {
			initializeDiskAction := &powershellAction{
				commandArgs: []string{
					"Initialize-Disk",
					"-Number",
					devicePath,
					"-PartitionStyle",
					"GPT",
				},
				commandFailureFmt: fmt.Sprintf("Failed to initialize disk %s: %%s", devicePath),
				cmdRunner:         p.cmdRunner,
			}

			_, err = initializeDiskAction.run()

			if err != nil {
				return err
			}
		}
	}

	existingTarget, err := p.linker.IsLinked(dataPath)

	if err != nil {
		return err
	}

	if existingTarget != "" {
		return nil
	}

	checkFreeSpaceAction := &powershellAction{
		commandArgs: []string{
			"Get-Disk",
			devicePath,
			"|",
			"Select",
			"-ExpandProperty",
			"LargestFreeExtent",
		},
		commandFailureFmt: fmt.Sprintf("Failed to get free disk space on disk %s: %%s", devicePath),
		cmdRunner:         p.cmdRunner,
	}

	freeSpaceOutput, err := checkFreeSpaceAction.run()

	if err != nil {
		return err
	}

	freeSpace, _ := strconv.Atoi(strings.TrimSpace(freeSpaceOutput))
	if freeSpace < 1024*1024 {
		p.logger.Warn(
			"WindowsPlatform",
			"Unable to create ephemeral partition on disk %s, as there isn't enough free space",
			devicePath,
		)
		return nil
	}

	partitionVolumeAction := &powershellAction{
		commandArgs: []string{
			"New-Partition",
			"-DiskNumber",
			devicePath,
			"-UseMaximumSize",
			"|",
			"Select",
			"-ExpandProperty",
			"PartitionNumber",
		},
		commandFailureFmt: fmt.Sprintf("Failed to create partition on disk %s: %%s", devicePath),
		cmdRunner:         p.cmdRunner,
	}

	partitionNumberOutput, err := partitionVolumeAction.run()
	if err != nil {
		return err
	}

	partitionNumber := strings.TrimSpace(partitionNumberOutput)

	err = p.formatter.Format(devicePath, partitionNumber)
	if err != nil {
		return err
	}

	mountVolumeAction := &powershellAction{
		commandArgs: []string{
			"Add-PartitionAccessPath",
			"-DiskNumber",
			devicePath,
			"-PartitionNumber",
			partitionNumber,
			"-AssignDriveLetter",
		},
		commandFailureFmt: fmt.Sprintf("Failed to assign drive letter to partition %s for device %s: %%s", partitionNumber, devicePath),
		cmdRunner:         p.cmdRunner,
	}

	_, err = mountVolumeAction.run()
	if err != nil {
		return err
	}

	getDriveLetterAction := &powershellAction{
		commandArgs: []string{
			"Get-Partition",
			"-DiskNumber",
			devicePath,
			"-PartitionNumber",
			partitionNumber,
			"|",
			"Select",
			"-ExpandProperty",
			"DriveLetter",
		},
		commandFailureFmt: fmt.Sprintf("Failed to retrieve drive letter for partition %s on disk %s: %%s", partitionNumber, devicePath),
		cmdRunner:         p.cmdRunner,
	}

	driveLetterOutput, err := getDriveLetterAction.run()
	if err != nil {
		return err
	}

	driveLetter := strings.TrimSpace(driveLetterOutput)

	mkLinkAction := &powershellAction{
		commandArgs: []string{
			"cmd.exe",
			"/c",
			"mklink",
			"/D",
			dataPath,
			fmt.Sprintf("%s:", driveLetter),
		},
		commandFailureFmt: fmt.Sprintf("Failed to link %s to %s: %%s", driveLetter, dataPath),
		cmdRunner:         p.cmdRunner,
	}

	_, err = mkLinkAction.run()
	if err != nil {
		return err
	}

	protectDataDirAction := &powershellAction{
		commandArgs: []string{
			"Protect-Path",
			fmt.Sprintf(`'%s'`, strings.TrimRight(dataPath, "\\")),
		},
		commandFailureFmt: fmt.Sprintf("Failed to protect path %s : %%s", dataPath),
		cmdRunner:         p.cmdRunner,
	}

	_, err = protectDataDirAction.run()
	if err != nil {
		return err
	}

	return nil
}

func (p WindowsPlatform) SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error) {
	return
}

func (p WindowsPlatform) SetupDataDir() error {
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

func (p WindowsPlatform) MountPersistentDisk(diskSettings boshsettings.DiskSettings, mountPoint string) (err error) {
	return
}

func (p WindowsPlatform) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (didUnmount bool, err error) {
	return
}

func (p WindowsPlatform) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) (diskPath string) {
	p.logger.Debug("WindowsPlatform", "Identifying ephemeral disk path, diskSettings.Path: `%s`", diskSettings.Path)

	if diskSettings.Path == "" && p.options.Linux.CreatePartitionIfNoEphemeralDisk {
		diskPath = "0"
	}

	if diskSettings.Path != "" {
		matchInt, _ := regexp.MatchString(`\d`, diskSettings.Path)
		if matchInt {
			diskPath = diskSettings.Path
		} else {
			alphs := []byte("abcdefghijklmnopq")

			lastChar := diskSettings.Path[len(diskSettings.Path)-1:]
			diskPath = fmt.Sprintf("%d", bytes.IndexByte(alphs, byte(lastChar[0])))
		}
	}

	p.logger.Debug("WindowsPlatform", "Identified Disk Path as `%s`", diskPath)

	return diskPath
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

func (p WindowsPlatform) GetDefaultNetwork() (boshsettings.Network, error) {
	return p.defaultNetworkResolver.GetDefaultNetwork()
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
	keypath := filepath.Join(sshdir, "ssh_host_ecdsa_key.pub")

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
		return "", bosherr.WrapErrorf(err, "Missing host public ECDSA key: %s", keypath)
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

func (p WindowsPlatform) Shutdown() error {
	return nil
}

const powerShellCmd = "powershell.exe"

type powershellAction struct {
	commandArgs       []string
	commandFailureFmt string
	cmdRunner         boshsys.CmdRunner
}

func (a *powershellAction) run() (string, error) {
	stdout, stderr, exitStatus, err := a.cmdRunner.RunCommand(powerShellCmd, a.commandArgs...)

	if err != nil {
		return "", fmt.Errorf("Failed to run command \"%s\": %s", strings.Join(
			append([]string{powerShellCmd}, a.commandArgs...), " "), err)
	}

	if exitStatus != 0 {
		return "", fmt.Errorf(a.commandFailureFmt, stderr)
	}

	return stdout, nil
}
