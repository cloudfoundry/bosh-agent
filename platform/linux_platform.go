package platform

import (
	"bytes"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"text/template"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"
	boshdpresolv "github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	"github.com/cloudfoundry/bosh-agent/v2/platform/cdrom"
	boshcert "github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const (
	ephemeralDiskPermissions  = os.FileMode(0750)
	persistentDiskPermissions = os.FileMode(0700)

	logDirPermissions         = os.FileMode(0750)
	runDirPermissions         = os.FileMode(0750)
	jobsDirPermissions        = os.FileMode(0750)
	packagesDirPermissions    = os.FileMode(0755)
	userBaseDirPermissions    = os.FileMode(0755)
	disksDirPermissions       = os.FileMode(0755)
	userRootLogDirPermissions = os.FileMode(0775)
	userRootOptDirPermissions = os.FileMode(0755)
	tmpDirPermissions         = os.FileMode(0755) // 0755 to make sure that vcap user can use new temp dir
	blobsDirPermissions       = os.FileMode(0700)

	sshDirPermissions          = os.FileMode(0700)
	sshAuthKeysFilePermissions = os.FileMode(0600)

	minRootEphemeralSpaceInBytes = uint64(1024 * 1024 * 1024)
)

type LinuxOptions struct {
	// When set to true loop back device
	// is not going to be overlayed over /tmp to limit /tmp dir size
	UseDefaultTmpDir bool

	// When set to true persistent disk will be assumed to be pre-formatted;
	// otherwise agent will partition and format it right before mounting
	UsePreformattedPersistentDisk bool

	// When set to true persistent disk will be mounted as a bind-mount
	BindMountPersistentDisk bool

	// When set to true and no ephemeral disk is mounted, the agent will create
	// a partition on the same device as the root partition to use as the
	// ephemeral disk
	CreatePartitionIfNoEphemeralDisk bool

	// When set to true the agent will skip both root and ephemeral disk partitioning
	SkipDiskSetup bool

	// Strategy for resolving device paths;
	// possible values: virtio, scsi, iscsi, ""
	DevicePathResolutionType string

	// Strategy for resolving ephemeral & persistent disk partitioners;
	// possible values: parted, "" (default is sfdisk if disk < 2TB, parted otherwise)
	PartitionerType string

	// Strategy for choosing service manager
	// possible values: systemd, ""
	ServiceManager string

	// Regular expression specifying what part of disk ID to strip and transform
	// example: "pattern": "^(disk-.+)$", "replacement": "google-${1}",
	DiskIDTransformPattern     string
	DiskIDTransformReplacement string

	// Base path for LUN-based symlink resolution (e.g., "/dev/disk/azure/data/by-lun").
	LunDeviceSymlinkPath string
}

type linux struct {
	fs                     boshsys.FileSystem
	cmdRunner              boshsys.CmdRunner
	collector              boshstats.Collector
	compressor             boshcmd.Compressor
	copier                 boshcmd.Copier
	dirProvider            boshdirs.Provider
	vitalsService          boshvitals.Service
	cdutil                 cdrom.CDUtil
	diskManager            boshdisk.Manager
	netManager             boshnet.Manager
	certManager            boshcert.Manager
	monitRetryStrategy     boshretry.RetryStrategy
	devicePathResolver     boshdpresolv.DevicePathResolver
	options                LinuxOptions
	state                  *BootstrapState
	logger                 boshlog.Logger
	defaultNetworkResolver boshsettings.DefaultNetworkResolver
	uuidGenerator          boshuuid.Generator
	auditLogger            AuditLogger
	logsTarProvider        boshlogstarprovider.LogsTarProvider
	serviceManager         servicemanager.ServiceManager
}

func NewLinuxPlatform(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	collector boshstats.Collector,
	compressor boshcmd.Compressor,
	copier boshcmd.Copier,
	dirProvider boshdirs.Provider,
	vitalsService boshvitals.Service,
	cdutil cdrom.CDUtil,
	diskManager boshdisk.Manager,
	netManager boshnet.Manager,
	certManager boshcert.Manager,
	monitRetryStrategy boshretry.RetryStrategy,
	devicePathResolver boshdpresolv.DevicePathResolver,
	state *BootstrapState,
	options LinuxOptions,
	logger boshlog.Logger,
	defaultNetworkResolver boshsettings.DefaultNetworkResolver,
	uuidGenerator boshuuid.Generator,
	auditLogger AuditLogger,
	logsTarProvider boshlogstarprovider.LogsTarProvider,
	serviceManager servicemanager.ServiceManager,
) Platform {
	return &linux{
		fs:                     fs,
		cmdRunner:              cmdRunner,
		collector:              collector,
		compressor:             compressor,
		copier:                 copier,
		dirProvider:            dirProvider,
		vitalsService:          vitalsService,
		cdutil:                 cdutil,
		diskManager:            diskManager,
		netManager:             netManager,
		certManager:            certManager,
		monitRetryStrategy:     monitRetryStrategy,
		devicePathResolver:     devicePathResolver,
		state:                  state,
		options:                options,
		logger:                 logger,
		defaultNetworkResolver: defaultNetworkResolver,
		uuidGenerator:          uuidGenerator,
		auditLogger:            auditLogger,
		logsTarProvider:        logsTarProvider,
		serviceManager:         serviceManager,
	}
}

const logTag = "linuxPlatform"

func (p linux) AssociateDisk(name string, settings boshsettings.DiskSettings) error {
	disksDir := p.dirProvider.DisksDir()
	err := p.fs.MkdirAll(disksDir, disksDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Associating disk: ")
	}

	linkPath := path.Join(disksDir, name)

	devicePath, _, err := p.devicePathResolver.GetRealDevicePath(settings)
	if err != nil {
		return bosherr.WrapErrorf(err, "Associating disk with name %s", name)
	}

	return p.fs.Symlink(devicePath, linkPath)
}

func (p linux) GetFs() (fs boshsys.FileSystem) {
	return p.fs
}

func (p linux) GetRunner() (runner boshsys.CmdRunner) {
	return p.cmdRunner
}

func (p linux) GetCompressor() (runner boshcmd.Compressor) {
	return p.compressor
}

func (p linux) GetCopier() (runner boshcmd.Copier) {
	return p.copier
}

func (p linux) GetLogsTarProvider() (logsTarProvider boshlogstarprovider.LogsTarProvider) {
	return p.logsTarProvider
}

func (p linux) GetDirProvider() (dirProvider boshdirs.Provider) {
	return p.dirProvider
}

func (p linux) GetVitalsService() (service boshvitals.Service) {
	return p.vitalsService
}

func (p linux) GetServiceManager() servicemanager.ServiceManager {
	return p.serviceManager
}

func (p linux) GetFileContentsFromCDROM(fileName string) (content []byte, err error) {
	contents, err := p.cdutil.GetFilesContents([]string{fileName})
	if err != nil {
		return []byte{}, err
	}

	return contents[0], nil
}

func (p linux) GetFilesContentsFromDisk(diskPath string, fileNames []string) ([][]byte, error) {
	return p.diskManager.GetUtil().GetFilesContents(diskPath, fileNames)
}

func (p linux) GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver) {
	return p.devicePathResolver
}

func (p linux) GetAuditLogger() AuditLogger {
	return p.auditLogger
}

func (p linux) SetupNetworking(networks boshsettings.Networks, mbus string) (err error) {
	return p.netManager.SetupNetworking(networks, mbus, nil)
}

func (p linux) GetConfiguredNetworkInterfaces() ([]string, error) {
	return p.netManager.GetConfiguredNetworkInterfaces()
}

func (p linux) GetCertManager() boshcert.Manager {
	return p.certManager
}

func (p linux) GetHostPublicKey() (string, error) {
	hostPublicKeyPath := "/etc/ssh/ssh_host_rsa_key.pub"
	hostPublicKey, err := p.fs.ReadFileString(hostPublicKeyPath)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "Unable to read host public key file: %s", hostPublicKeyPath)
	}
	return hostPublicKey, nil
}

func (p linux) SetupRuntimeConfiguration() (err error) {
	_, _, _, err = p.cmdRunner.RunCommand("bosh-agent-rc")
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to bosh-agent-rc")
	}
	return
}

func (p linux) CreateUser(username, basePath string) error {
	err := p.fs.MkdirAll(basePath, userBaseDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Making user base path")
	}

	args := []string{"-m", "-b", basePath, "-s", "/bin/bash", username}

	_, _, _, err = p.cmdRunner.RunCommand("useradd", args...)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to useradd")
	}

	userHomeDir, err := p.fs.HomeDir(username)
	if err != nil {
		return bosherr.WrapErrorf(err, "Unable to retrieve home directory for user %s", username)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "700", userHomeDir)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to chmod")
	}

	return nil
}

func (p linux) AddUserToGroups(username string, groups []string) error {
	_, _, _, err := p.cmdRunner.RunCommand("usermod", "-G", strings.Join(groups, ","), username)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to usermod")
	}
	return nil
}

func (p linux) DeleteEphemeralUsersMatching(reg string) error {
	compiledReg, err := regexp.Compile(reg)
	if err != nil {
		return bosherr.WrapError(err, "Compiling regexp")
	}

	matchingUsers, err := p.findEphemeralUsersMatching(compiledReg)
	if err != nil {
		return bosherr.WrapError(err, "Finding ephemeral users")
	}

	for _, user := range matchingUsers {
		err = p.deleteUser(user)
		if err != nil {
			return bosherr.WrapError(err, "Deleting user")
		}
	}
	return nil
}

func (p linux) deleteUser(user string) (err error) {
	_, _, _, err = p.cmdRunner.RunCommand("userdel", "-rf", user)
	return
}

func (p linux) findEphemeralUsersMatching(reg *regexp.Regexp) (matchingUsers []string, err error) {
	passwd, err := p.fs.ReadFileString("/etc/passwd")
	if err != nil {
		err = bosherr.WrapError(err, "Reading /etc/passwd")
		return
	}

	for _, line := range strings.Split(passwd, "\n") {
		user := strings.Split(line, ":")[0]
		matchesPrefix := strings.HasPrefix(user, boshsettings.EphemeralUserPrefix)
		matchesReg := reg.MatchString(user)

		if matchesPrefix && matchesReg {
			matchingUsers = append(matchingUsers, user)
		}
	}
	return
}

func (p linux) SetupBoshSettingsDisk() (err error) {
	agentSettingsTmpfsDir := filepath.Dir(p.GetAgentSettingsPath(true))

	err = p.fs.MkdirAll(agentSettingsTmpfsDir, 0700)
	if err != nil {
		err = bosherr.WrapError(err, "Setting up Bosh Settings Disk")
		return
	}
	return p.diskManager.GetMounter().MountTmpfs(agentSettingsTmpfsDir, "16m")
}

func (p linux) GetAgentSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "settings.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "settings.json")
}

func (p linux) GetPersistentDiskSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "persistent_disk_hints.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "persistent_disk_hints.json")
}

func (p linux) GetUpdateSettingsPath(tmpfs bool) string {
	if tmpfs {
		return filepath.Join(p.dirProvider.BoshSettingsDir(), "update_settings.json")
	}
	return filepath.Join(p.dirProvider.BoshDir(), "update_settings.json")
}

func (p linux) SetupRootDisk(ephemeralDiskPath string) error {
	var resizeCmd string
	var resizeCmdArgs []string

	if p.options.SkipDiskSetup {
		return nil
	}

	// if there is ephemeral disk we can safely autogrow, if not we should not.
	if (ephemeralDiskPath == "") && p.options.CreatePartitionIfNoEphemeralDisk {
		p.logger.Info(logTag, "No Ephemeral Disk provided, Skipping growing of the Root Filesystem")
		return nil
	}

	// in case growpart is not available for another flavour of linux, don't stop the agent from running,
	// without this integration-test would not run since the bosh-lite vm doesn't have it
	if !p.cmdRunner.CommandExists("growpart") {
		p.logger.Info(logTag, "The program 'growpart' is not installed, Root Filesystem cannot be grown")
		return nil
	}

	rootDevicePath, rootDeviceNumber, err := p.findRootDevicePathAndNumber()
	if err != nil {
		return bosherr.WrapError(err, "findRootDevicePath")
	}

	stdout, _, _, err := p.cmdRunner.RunCommand(
		"growpart",
		rootDevicePath,
		strconv.Itoa(rootDeviceNumber),
	)

	if err != nil {
		if !strings.Contains(stdout, "NOCHANGE") {
			return bosherr.WrapError(err, "growpart")
		}
	}

	rootDevice := p.partitionPath(rootDevicePath, rootDeviceNumber)
	fsType, err := p.diskManager.GetFormatter().GetPartitionFormatType(rootDevice)
	if err != nil {
		return bosherr.WrapError(err, "Getting root partition filesystem type")
	}

	switch fsType {
	case boshdisk.FileSystemXFS:
		resizeCmd = boshdisk.FileSystemXFSResizeUtility
		resizeCmdArgs = []string{"-d", rootDevice}
	case boshdisk.FileSystemExt4:
		resizeCmd = boshdisk.FileSystemExtResizeUtility
		resizeCmdArgs = []string{"-f", rootDevice}
	default:
		return bosherr.Errorf("Cannot get filesystem type for root file system")
	}
	_, _, _, err = p.cmdRunner.RunComplexCommand(boshsys.Command{
		Name: resizeCmd,
		Args: resizeCmdArgs,
	})

	if err != nil {
		return bosherr.WrapError(err, resizeCmd)
	}

	return nil
}

func (p linux) SetupSSH(publicKeys []string, username string) error {
	homeDir, err := p.fs.HomeDir(username)
	if err != nil {
		return bosherr.WrapError(err, "Finding home dir for user")
	}

	sshPath := path.Join(homeDir, ".ssh")
	err = p.fs.MkdirAll(sshPath, sshDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Making ssh directory")
	}
	err = p.fs.Chown(sshPath, username)
	if err != nil {
		return bosherr.WrapError(err, "Chowning ssh directory")
	}

	authKeysPath := path.Join(sshPath, "authorized_keys")
	publicKeyString := strings.Join(publicKeys, "\n")
	err = p.fs.WriteFileString(authKeysPath, publicKeyString)
	if err != nil {
		return bosherr.WrapError(err, "Creating authorized_keys file")
	}

	err = p.fs.Chown(authKeysPath, username)
	if err != nil {
		return bosherr.WrapError(err, "Chowning key path")
	}
	err = p.fs.Chmod(authKeysPath, sshAuthKeysFilePermissions)
	if err != nil {
		return bosherr.WrapError(err, "Chmoding key path")
	}

	return nil
}

func (p linux) SetUserPassword(user, encryptedPwd string) (err error) {
	if encryptedPwd == "" {
		encryptedPwd = "*"
	}
	_, _, _, err = p.cmdRunner.RunCommand("usermod", "-p", encryptedPwd, user)
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to usermod")
	}
	return
}

func (p linux) SetupRecordsJSONPermission(path string) error {
	if err := p.fs.Chmod(path, 0640); err != nil {
		return bosherr.WrapError(err, "Chmoding records JSON file")
	}

	if err := p.fs.Chown(path, "root:vcap"); err != nil {
		return bosherr.WrapError(err, "Chowning records JSON file")
	}

	return nil
}

const EtcHostsTemplate = `127.0.0.1 {{ . }} localhost

# The following lines are desirable for IPv6 capable hosts
::1 {{ . }} localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
`

func (p linux) SaveDNSRecords(dnsRecords boshsettings.DNSRecords, hostname string) error {
	dnsRecordsContents, err := p.generateDefaultEtcHosts(hostname)
	if err != nil {
		return bosherr.WrapError(err, "Generating default /etc/hosts")
	}

	for _, dnsRecord := range dnsRecords.Records {
		dnsRecordsContents.WriteString(fmt.Sprintf("%s %s\n", dnsRecord[0], dnsRecord[1])) //nolint:staticcheck
	}

	uuid, err := p.uuidGenerator.Generate()
	if err != nil {
		return bosherr.WrapError(err, "Generating UUID")
	}

	etcHostsUUIDFileName := fmt.Sprintf("/etc/hosts-%s", uuid)
	err = p.fs.WriteFileQuietly(etcHostsUUIDFileName, dnsRecordsContents.Bytes())
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Writing to %s", etcHostsUUIDFileName))
	}

	err = p.fs.Rename(etcHostsUUIDFileName, "/etc/hosts")
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Renaming %s to /etc/hosts", etcHostsUUIDFileName))
	}

	return nil
}

func (p linux) SetupIPv6(config boshsettings.IPv6) error {
	return p.netManager.SetupIPv6(config, nil)
}

func (p linux) SetupHostname(hostname string) error {
	if p.state.Linux.HostsConfigured {
		return nil
	}

	_, _, _, err := p.cmdRunner.RunCommand("hostname", hostname)
	if err != nil {
		return bosherr.WrapError(err, "Setting hostname")
	}

	err = p.fs.WriteFileString("/etc/hostname", hostname)
	if err != nil {
		return bosherr.WrapError(err, "Writing to /etc/hostname")
	}

	buffer, err := p.generateDefaultEtcHosts(hostname)
	if err != nil {
		return err
	}
	err = p.fs.WriteFile("/etc/hosts", buffer.Bytes())
	if err != nil {
		return bosherr.WrapError(err, "Writing to /etc/hosts")
	}

	p.state.Linux.HostsConfigured = true
	err = p.state.SaveState()
	if err != nil {
		return bosherr.WrapError(err, "Setting up hostname")
	}

	return nil
}

func (p linux) SetupLogrotate(groupName, basePath, size string) (err error) {
	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("logrotate-d-config").Parse(etcLogrotateDTemplate))

	type logrotateArgs struct {
		BasePath string
		Size     string
	}

	err = t.Execute(buffer, logrotateArgs{basePath, size})
	if err != nil {
		err = bosherr.WrapError(err, "Generating logrotate config")
		return
	}

	err = p.fs.WriteFile(path.Join("/etc/logrotate.d", groupName), buffer.Bytes())
	if err != nil {
		err = bosherr.WrapError(err, "Writing to /etc/logrotate.d")
		return
	}

	_, _, _, _ = p.cmdRunner.RunCommand("/var/vcap/bosh/bin/setup-logrotate.sh") //nolint:errcheck
	return
}

// Logrotate config file - /etc/logrotate.d/<group-name>
// Stemcell stage logrotate_config configures logrotate to run every hour
const etcLogrotateDTemplate = `# Generated by bosh-agent

{{ .BasePath }}/data/sys/log/*.log {{ .BasePath }}/data/sys/log/.*.log {{ .BasePath }}/data/sys/log/*/*.log {{ .BasePath }}/data/sys/log/*/.*.log {{ .BasePath }}/data/sys/log/*/*/*.log {{ .BasePath }}/data/sys/log/*/*/.*.log {
  missingok
  rotate 7
  compress
  copytruncate
  size={{ .Size }}
}
`

func (p linux) SetTimeWithNtpServers(servers []string) (err error) {
	serversFilePath := path.Join(p.dirProvider.BaseDir(), "/bosh/etc/ntpserver")
	if len(servers) == 0 {
		return
	}

	err = p.fs.WriteFileString(serversFilePath, strings.Join(servers, " "))
	if err != nil {
		err = bosherr.WrapErrorf(err, "Writing to %s", serversFilePath)
		return
	}

	// Make a best effort to sync time now but don't error
	_, _, _, _ = p.cmdRunner.RunCommand("sync-time") //nolint:errcheck
	return
}

func (p linux) SetupEphemeralDiskWithPath(realPath string, desiredSwapSizeInBytes *uint64, labelPrefix string) error {
	p.logger.Info(logTag, "Setting up ephemeral disk...")
	mountPoint := p.dirProvider.DataDir()

	mountPointGlob := path.Join(mountPoint, "*")
	contents, err := p.fs.Glob(mountPointGlob)
	if err != nil {
		return bosherr.WrapErrorf(err, "Globbing ephemeral disk mount point `%s'", mountPointGlob)
	}

	if len(contents) > 0 {
		// When agent bootstraps for the first time data directory should be empty.
		// It might be non-empty on subsequent agent restarts. The ephemeral disk setup
		// should be idempotent and partitioning will be skipped if disk is already
		// partitioned as needed. If disk is not partitioned as needed we still want to
		// partition it even if data directory is not empty.
		p.logger.Debug(logTag, "Existing ephemeral mount `%s' is not empty. Contents: %s", mountPoint, contents)
	}

	err = p.fs.MkdirAll(mountPoint, ephemeralDiskPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating data dir")
	}

	if p.options.SkipDiskSetup {
		return nil
	}

	var swapPartitionPath, dataPartitionPath string

	// Agent can only setup ephemeral data directory either on ephemeral device
	// or on separate root partition.
	// The real path can be empty if CPI did not provide ephemeral disk
	// or if the provided disk was not found.
	if realPath == "" {
		if !p.options.CreatePartitionIfNoEphemeralDisk {
			// Agent can not use root partition for ephemeral data directory.
			return bosherr.Error("No ephemeral disk found, cannot use root partition as ephemeral disk")
		}

		swapPartitionPath, dataPartitionPath, err = p.createEphemeralPartitionsOnRootDevice(desiredSwapSizeInBytes, labelPrefix)
		if err != nil {
			return bosherr.WrapError(err, "Creating ephemeral partitions on root device")
		}
	} else {
		swapPartitionPath, dataPartitionPath, err = p.partitionEphemeralDisk(realPath, desiredSwapSizeInBytes, labelPrefix)
		if err != nil {
			return bosherr.WrapError(err, "Partitioning ephemeral disk")
		}
	}

	if len(swapPartitionPath) > 0 {
		canonicalSwapPartitionPath, err := resolveCanonicalLink(p.cmdRunner, swapPartitionPath)
		if err != nil {
			return err
		}

		p.logger.Info(logTag, "Formatting `%s' (canonical path: %s) as swap", swapPartitionPath, canonicalSwapPartitionPath)
		err = p.diskManager.GetFormatter().Format(canonicalSwapPartitionPath, boshdisk.FileSystemSwap)
		if err != nil {
			return bosherr.WrapError(err, "Formatting swap")
		}

		p.logger.Info(logTag, "Mounting `%s' (canonical path: %s) as swap", swapPartitionPath, canonicalSwapPartitionPath)
		err = p.diskManager.GetMounter().SwapOn(canonicalSwapPartitionPath)
		if err != nil {
			return bosherr.WrapError(err, "Mounting swap")
		}
	}

	canonicalDataPartitionPath, err := resolveCanonicalLink(p.cmdRunner, dataPartitionPath)
	if err != nil {
		return err
	}

	p.logger.Info(logTag, "Formatting `%s' (canonical path: %s) as ext4", dataPartitionPath, canonicalDataPartitionPath)
	err = p.diskManager.GetFormatter().Format(canonicalDataPartitionPath, boshdisk.FileSystemExt4)
	if err != nil {
		return bosherr.WrapError(err, "Formatting data partition with ext4")
	}

	p.logger.Info(logTag, "Mounting `%s' (canonical path: %s) at `%s'", dataPartitionPath, canonicalDataPartitionPath, mountPoint)
	err = p.diskManager.GetMounter().Mount(canonicalDataPartitionPath, mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Mounting data partition")
	}

	return nil
}

func (p linux) SetupRawEphemeralDisks(devices []boshsettings.DiskSettings) (err error) {
	if p.options.SkipDiskSetup {
		return nil
	}

	p.logger.Info(logTag, "Setting up raw ephemeral disks")

	for i, device := range devices {
		realPath, _, err := p.devicePathResolver.GetRealDevicePath(device)
		if err != nil {
			return bosherr.WrapError(err, "Getting real device path")
		}

		// check if device is already partitioned correctly
		stdout, stderr, _, err := p.cmdRunner.RunCommand(
			"parted",
			"-s",
			realPath,
			"p",
		)

		if err != nil {
			// "unrecognised disk label" is acceptable, since the disk may not have been partitioned
			if !strings.Contains(stdout, "unrecognised disk label") &&
				!strings.Contains(stderr, "unrecognised disk label") {
				return bosherr.WrapError(err, "Setting up raw ephemeral disks")
			}
		}

		if strings.Contains(stdout, "Partition Table: gpt") &&
			strings.Contains(stdout, "raw-ephemeral-") {
			continue
		}

		// change to gpt partition type, change units to percentage, make partition with name and span from 0-100%
		p.logger.Info(logTag, "Creating partition on `%s'", realPath)
		_, _, _, err = p.cmdRunner.RunCommand(
			"parted",
			"-s",
			realPath,
			"mklabel",
			"gpt",
			"unit",
			"%",
			"mkpart",
			fmt.Sprintf("raw-ephemeral-%d", i),
			"0",
			"100",
		)

		if err != nil {
			return bosherr.WrapError(err, "Setting up raw ephemeral disks")
		}
	}

	return nil
}

func (p linux) SetupDataDir(jobConfig boshsettings.JobDir, runConfig boshsettings.RunDir) error {
	dataDir := p.dirProvider.DataDir()

	sysDataDir := path.Join(dataDir, "sys")

	logDir := path.Join(sysDataDir, "log")
	err := p.fs.MkdirAll(logDir, logDirPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", logDir)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", sysDataDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", sysDataDir)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", logDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", logDir)
	}

	jobsDir := p.dirProvider.DataJobsDir()
	err = p.fs.MkdirAll(jobsDir, jobsDirPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", jobsDir)
	}

	sensitiveDir := p.dirProvider.SensitiveBlobsDir()
	err = p.fs.MkdirAll(sensitiveDir, blobsDirPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", sensitiveDir)
	}

	if jobConfig.TmpFS {
		size := jobConfig.TmpFSSize
		if size == "" {
			size = "100m"
		}

		if err = p.diskManager.GetMounter().MountTmpfs(jobsDir, size); err != nil {
			return err
		}
		if err = p.diskManager.GetMounter().MountTmpfs(sensitiveDir, size); err != nil {
			return err
		}
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", jobsDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", jobsDir)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", sensitiveDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", sensitiveDir)
	}

	packagesDir := p.dirProvider.PkgDir()
	err = p.fs.MkdirAll(packagesDir, packagesDirPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Making %s dir", packagesDir)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", packagesDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", packagesDir)
	}

	err = p.setupRunDir(sysDataDir, runConfig.TmpFSSize)
	if err != nil {
		return err
	}

	sysDir := path.Join(path.Dir(dataDir), "sys")
	err = p.fs.Symlink(sysDataDir, sysDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Symlinking '%s' to '%s'", sysDir, sysDataDir)
	}

	return nil
}

func (p linux) setupRunDir(sysDir, tmppFSSize string) error {
	runDir := path.Join(sysDir, "run")

	_, runDirIsMounted, err := p.IsMountPoint(runDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Checking for mount point %s", runDir)
	}

	if !runDirIsMounted {
		err = p.fs.MkdirAll(runDir, runDirPermissions)
		if err != nil {
			return bosherr.WrapErrorf(err, "Making %s dir", runDir)
		}

		if tmppFSSize == "" {
			tmppFSSize = "16m"
		}

		err = p.diskManager.GetMounter().MountTmpfs(runDir, tmppFSSize)
		if err != nil {
			return bosherr.WrapErrorf(err, "Mounting tmpfs to %s", runDir)
		}

		_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", runDir)
		if err != nil {
			return bosherr.WrapErrorf(err, "chown %s", runDir)
		}
	}

	return nil
}

func (p linux) SetupHomeDir() error {
	mounter := boshdisk.NewLinuxBindMounter(p.diskManager.GetMounter())
	isMounted, err := mounter.IsMounted("/home")
	if err != nil {
		return bosherr.WrapError(err, "Setup home dir, checking if mounted")
	}
	if !isMounted {
		err := mounter.Mount("/home", "/home")
		if err != nil {
			return bosherr.WrapError(err, "Setup home dir, mounting home")
		}
		err = mounter.RemountInPlace("/home", "nodev", "nosuid")
		if err != nil {
			return bosherr.WrapError(err, "Setup home dir, remount in place")
		}
	}
	return nil
}

func (p linux) SetupBlobsDir() error {
	blobsDirPath := p.dirProvider.BlobsDir()

	err := p.fs.MkdirAll(blobsDirPath, blobsDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating blobs dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", blobsDirPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", blobsDirPath)
	}

	return nil
}

func (p linux) SetupCanRestartDir() error {
	canRebootDir := p.dirProvider.CanRestartDir()

	err := p.fs.MkdirAll(canRebootDir, 0740)
	if err != nil {
		return bosherr.WrapError(err, "Creating canReboot dir")
	}

	if err = p.diskManager.GetMounter().MountTmpfs(canRebootDir, "16m"); err != nil {
		return err
	}
	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:vcap", canRebootDir)
	if err != nil {
		return bosherr.WrapError(err, "Chowning canrestart dir")
	}

	return nil
}

func (p linux) SetupTmpDir() error {
	systemTmpDir := "/tmp"
	boshTmpDir := p.dirProvider.TmpDir()
	boshRootTmpPath := path.Join(p.dirProvider.DataDir(), "root_tmp")

	err := p.fs.MkdirAll(boshTmpDir, tmpDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating temp dir")
	}

	err = os.Setenv("TMPDIR", boshTmpDir)
	if err != nil {
		return bosherr.WrapError(err, "Setting TMPDIR")
	}

	err = p.changeTmpDirPermissions(systemTmpDir)
	if err != nil {
		return err
	}

	// /var/tmp is used for preserving temporary files between system reboots
	varTmpDir := "/var/tmp"

	err = p.changeTmpDirPermissions(varTmpDir)
	if err != nil {
		return err
	}

	if p.options.UseDefaultTmpDir {
		return nil
	}

	_, _, _, err = p.cmdRunner.RunCommand("mkdir", "-p", boshRootTmpPath)
	if err != nil {
		return bosherr.WrapError(err, "Creating root tmp dir")
	}

	err = p.changeTmpDirPermissions(boshRootTmpPath)
	if err != nil {
		return bosherr.WrapError(err, "Chmoding root tmp dir")
	}

	err = p.bindMountDir(boshRootTmpPath, systemTmpDir, false, false)
	if err != nil {
		return err
	}

	err = p.bindMountDir(boshRootTmpPath, varTmpDir, false, false)
	if err != nil {
		return err
	}

	return nil
}

func (p linux) SetupSharedMemory() error {
	for _, mnt := range []string{"/dev/shm", "/run/shm"} {
		err := p.remountWithSecurityFlags(mnt)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p linux) remountWithSecurityFlags(mountPt string) error {
	mounter := p.diskManager.GetMounter()

	_, mounted, err := mounter.IsMountPoint(mountPt)
	if err != nil {
		return err
	}

	if mounted {
		return mounter.RemountInPlace(mountPt, "noexec", "nodev", "nosuid")
	}

	return nil
}

func (p linux) SetupLogDir() error {
	logDir := "/var/log"

	boshRootLogPath := path.Join(p.dirProvider.DataDir(), "root_log")

	err := p.fs.MkdirAll(boshRootLogPath, userRootLogDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating root log dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "0771", boshRootLogPath)
	if err != nil {
		return bosherr.WrapError(err, "Chmoding /var/log dir")
	}

	auditDirPath := path.Join(boshRootLogPath, "audit")
	_, _, _, err = p.cmdRunner.RunCommand("mkdir", "-p", auditDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Creating audit log dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "0750", auditDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Chmoding audit log dir")
	}

	sysstatDirPath := path.Join(boshRootLogPath, "sysstat")
	_, _, _, err = p.cmdRunner.RunCommand("mkdir", "-p", sysstatDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Creating sysstat log dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "0755", sysstatDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Chmoding sysstat log dir")
	}

	// change ownership
	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:syslog", boshRootLogPath)
	if err != nil {
		return bosherr.WrapError(err, "Chowning root log dir")
	}

	err = p.ensureFile(fmt.Sprintf("%s/btmp", boshRootLogPath), "root:utmp", "0600")
	if err != nil {
		return err
	}

	err = p.ensureFile(fmt.Sprintf("%s/wtmp", boshRootLogPath), "root:utmp", "0664")
	if err != nil {
		return err
	}

	err = p.ensureFile(fmt.Sprintf("%s/lastlog", boshRootLogPath), "root:utmp", "0664")
	if err != nil {
		return err
	}

	err = p.bindMountDir(boshRootLogPath, logDir, false, false)
	if err != nil {
		return err
	}

	result, err := p.fs.ReadFileString("/etc/passwd")
	if err != nil {
		return nil
	}

	rx := regexp.MustCompile("(?m)^_chrony:")

	if rx.MatchString(result) {
		chronyDirPath := path.Join(boshRootLogPath, "chrony")
		_, _, _, err = p.cmdRunner.RunCommand("mkdir", "-p", chronyDirPath)
		if err != nil {
			return bosherr.WrapError(err, "Creating chrony log dir")
		}

		_, _, _, err = p.cmdRunner.RunCommand("chmod", "0700", chronyDirPath)
		if err != nil {
			return bosherr.WrapError(err, "Chmoding chrony log dir")
		}

		_, _, _, err = p.cmdRunner.RunCommand("chown", "_chrony:_chrony", chronyDirPath)
		if err != nil {
			return bosherr.WrapError(err, "Chowning chrony log dir")
		}
	}

	return nil
}

func (p linux) SetupOptDir() error {
	varOptDir := "/var/opt"

	boshRootVarOptDirPath := path.Join(p.dirProvider.DataDir(), "root_var_opt")
	err := p.fs.MkdirAll(boshRootVarOptDirPath, userRootOptDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating root var opt dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:root", boshRootVarOptDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Chowning root var opt dir")
	}

	// Mount our /var/opt bind mount without the 'noexec' option. Binaries are
	// often in subdirectories of /var/opt, and folks expect to be able to execute them.
	err = p.bindMountDir(boshRootVarOptDirPath, varOptDir, true, true)
	if err != nil {
		return err
	}

	optDir := "/opt"

	boshRootOptDirPath := path.Join(p.dirProvider.DataDir(), "root_opt")
	err = p.fs.MkdirAll(boshRootOptDirPath, userRootOptDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Creating root opt dir")
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", "root:root", boshRootOptDirPath)
	if err != nil {
		return bosherr.WrapError(err, "Chowning root opt dir")
	}

	// Mount our /opt bind mount without the 'noexec' option. Binaries are
	// often in subdirectories of /opt, and folks expect to be able to execute them.
	err = p.bindMountDir(boshRootOptDirPath, optDir, true, true)
	if err != nil {
		return err
	}

	return err
}

func (p linux) ensureFile(path, owner, mode string) error {
	_, _, _, err := p.cmdRunner.RunCommand("touch", path)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Touching '%s' file", path))
	}

	_, _, _, err = p.cmdRunner.RunCommand("chown", owner, path)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Chowning '%s' file", path))
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", mode, path)
	if err != nil {
		return bosherr.WrapError(err, fmt.Sprintf("Chmoding '%s' file", path))
	}

	return nil
}

func (p linux) SetupLoggingAndAuditing() error {
	_, _, _, err := p.cmdRunner.RunCommand("/var/vcap/bosh/bin/bosh-start-logging-and-auditing")
	if err != nil {
		return bosherr.WrapError(err, "Running start logging and audit script")
	}
	return nil
}

func (p linux) bindMountDir(mountSource, mountPoint string, allowExec bool, allowSuid bool) error {
	bindMounter := boshdisk.NewLinuxBindMounter(p.diskManager.GetMounter())
	mounted, err := bindMounter.IsMounted(mountPoint)

	if !mounted && err == nil {
		err = bindMounter.Mount(mountSource, mountPoint)
		if err != nil {
			return bosherr.WrapErrorf(err, "Bind mounting %s dir over %s", mountSource, mountPoint)
		}
	} else if err != nil {
		return err
	}

	mountOptions := []string{"nodev"}
	if !allowSuid {
		mountOptions = append(mountOptions, "nosuid")
	}
	if !allowExec {
		mountOptions = append(mountOptions, "noexec")
	}
	return bindMounter.RemountInPlace(mountPoint, mountOptions...)
}

func (p linux) changeTmpDirPermissions(path string) error {
	_, _, _, err := p.cmdRunner.RunCommand("chown", "root:vcap", path)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", path)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "1777", path)
	if err != nil {
		return bosherr.WrapErrorf(err, "chmod %s", path)
	}

	return nil
}

func (p linux) AdjustPersistentDiskPartitioning(diskSetting boshsettings.DiskSettings, mountPoint string) error {
	if p.options.UsePreformattedPersistentDisk {
		return nil
	}
	p.logger.Debug(logTag, "Adjusting size for persistent disk %+v", diskSetting)

	devicePath, _, err := p.devicePathResolver.GetRealDevicePath(diskSetting)
	if err != nil {
		return bosherr.WrapError(err, "Getting real device path")
	}

	firstPartitionPath := p.partitionPath(devicePath, 1)

	partitioner, err := p.diskManager.GetPersistentDevicePartitioner(diskSetting.Partitioner)
	if err != nil {
		return bosherr.WrapError(err, "Selecting partitioner")
	}

	singlePartNeedsResize, err := partitioner.SinglePartitionNeedsResize(devicePath, boshdisk.PartitionTypeLinux)
	if err != nil {
		return bosherr.WrapError(err, "Failed to determine whether partitions need rezising")
	}
	p.logger.Debug(logTag, "Persistent disk single partition needs resize: %+v", singlePartNeedsResize)

	if singlePartNeedsResize { //nolint:nestif
		err = partitioner.ResizeSinglePartition(devicePath)
		if err != nil {
			return bosherr.WrapError(err, "Resizing disk partition")
		}

		err := p.diskManager.GetMounter().Mount(firstPartitionPath, mountPoint, diskSetting.MountOptions...)
		if err != nil {
			return bosherr.WrapError(err, "Failed to mount partition for filesystem growing")
		}

		err = p.diskManager.GetFormatter().GrowFilesystem(firstPartitionPath)
		if err != nil {
			return bosherr.WrapError(err, "Failed to grow filesystem")
		}

		_, err = p.diskManager.GetMounter().Unmount(firstPartitionPath)
		if err != nil {
			return bosherr.WrapError(err, "Failed to unmount partition after filesystem growing")
		}
	} else {
		singlePartPartitioning := []boshdisk.Partition{
			{Type: boshdisk.PartitionTypeLinux},
		}
		err = partitioner.Partition(devicePath, singlePartPartitioning)
		if err != nil {
			return bosherr.WrapError(err, "Partitioning disk")
		}

		persistentDiskFS := diskSetting.FileSystemType
		switch persistentDiskFS {
		case boshdisk.FileSystemExt4, boshdisk.FileSystemXFS:
		case boshdisk.FileSystemDefault:
			persistentDiskFS = boshdisk.FileSystemExt4
		case boshdisk.FileSystemSwap:
			fallthrough
		default:
			return bosherr.Error(fmt.Sprintf(`The filesystem type "%s" is not supported`, diskSetting.FileSystemType))
		}

		err = p.diskManager.GetFormatter().Format(firstPartitionPath, persistentDiskFS)
		if err != nil {
			return bosherr.WrapError(err, fmt.Sprintf("Formatting partition with %s", diskSetting.FileSystemType))
		}
	}
	return nil
}

func (p linux) MountPersistentDisk(diskSetting boshsettings.DiskSettings, mountPoint string) error {
	p.logger.Debug(logTag, "Mounting persistent disk %+v at %s", diskSetting, mountPoint)

	devicePath, _, err := p.devicePathResolver.GetRealDevicePath(diskSetting)
	if err != nil {
		return bosherr.WrapError(err, "Getting real device path")
	}

	alreadyMountedPartPath, hasMountedDevice, err := p.IsMountPoint(mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Checking mount point already has a device monted onto")
	}
	p.logger.Info(logTag, "devicePath = %s, alreadyMountedPartPath = %s, hasMountedDevice = %t", devicePath, alreadyMountedPartPath, hasMountedDevice)

	firstPartitionPath := p.partitionPath(devicePath, 1)
	if hasMountedDevice {
		if alreadyMountedPartPath == firstPartitionPath {
			p.logger.Info(logTag, "device: %s is already mounted on %s, skipping mounting", alreadyMountedPartPath, mountPoint)
			return nil
		}

		mountPoint = p.dirProvider.StoreMigrationDir()
	}

	err = p.fs.MkdirAll(mountPoint, persistentDiskPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating directory %s", mountPoint)
	}

	var partitionPathToMount string
	if p.options.UsePreformattedPersistentDisk {
		partitionPathToMount = devicePath
	} else {
		partitionPathToMount = firstPartitionPath
	}

	err = p.diskManager.GetMounter().Mount(partitionPathToMount, mountPoint, diskSetting.MountOptions...)
	if err != nil {
		return bosherr.WrapError(err, "Mounting partition")
	}

	managedSettingsPath := filepath.Join(p.dirProvider.BoshDir(), "managed_disk_settings.json")

	err = p.fs.WriteFileString(managedSettingsPath, diskSetting.ID)
	if err != nil {
		return bosherr.WrapError(err, "Writing managed_disk_settings.json")
	}

	return nil
}

func (p linux) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (bool, error) {
	p.logger.Debug(logTag, "Unmounting persistent disk %+v", diskSettings)

	realPath, timedOut, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if timedOut {
		return false, nil
	}
	if err != nil {
		return false, bosherr.WrapError(err, "Getting real device path")
	}

	if !p.options.UsePreformattedPersistentDisk {
		realPath = p.partitionPath(realPath, 1)
	}

	return p.diskManager.GetMounter().Unmount(realPath)
}

func (p linux) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) (string, error) {
	realPath, _, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		p.logger.Debug(logTag, "Error getting ephermeral disk path %v", err)
		return "", nil
	}

	return realPath, nil
}

func (p linux) IsPersistentDiskMountable(diskSettings boshsettings.DiskSettings) (bool, error) {
	realPath, _, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		return false, bosherr.WrapErrorf(err, "Validating path: %s", diskSettings.Path)
	}

	stdout, stderr, _, _ := p.cmdRunner.RunCommand("sfdisk", "-d", realPath) //nolint:errcheck
	if strings.Contains(stderr, "unrecognized partition table type") {
		return false, nil
	}

	lines := len(strings.Split(stdout, "\n"))
	return lines > 4, nil
}

func (p linux) IsMountPoint(path string) (string, bool, error) {
	return p.diskManager.GetMounter().IsMountPoint(path)
}

func (p linux) MigratePersistentDisk(fromMountPoint, toMountPoint string) error {
	p.logger.Debug(logTag, "Migrating persistent disk %v to %v", fromMountPoint, toMountPoint)

	err := p.diskManager.GetMounter().RemountAsReadonly(fromMountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Remounting persistent disk as readonly")
	}

	// Golang does not implement a file copy that would allow us to preserve dates...
	// So we have to shell out to tar to perform the copy instead of delegating to the FileSystem
	// The --xattrs and --xattrs-include=*.* flags ensure that all extended attributes (ex. capabilities) are preserved
	tarCopy := fmt.Sprintf("(tar -C %s --xattrs --xattrs-include=*.* --sparse -cf - .) | (tar -C %s --xattrs --xattrs-include=*.* -xpf -)", fromMountPoint, toMountPoint)
	_, _, _, err = p.cmdRunner.RunCommand("sh", "-c", tarCopy)
	if err != nil {
		return bosherr.WrapError(err, "Copying files from old disk to new disk")
	}

	// Find iSCSI device id of fromMountPoint
	var iscsiID string
	if p.options.DevicePathResolutionType == "iscsi" {
		mounts, err := p.diskManager.GetMountsSearcher().SearchMounts()
		if err != nil {
			return bosherr.WrapError(err, "Search persistent disk as readonly")
		}

		devMapperPart1Regexp := regexp.MustCompile(`/dev/mapper/(.*?)-part1`)
		for _, mount := range mounts {
			if mount.MountPoint == fromMountPoint {
				matches := devMapperPart1Regexp.FindStringSubmatch(mount.PartitionPath)
				if len(matches) > 1 {
					iscsiID = matches[1]
				}
			}
		}
	}

	_, err = p.diskManager.GetMounter().Unmount(fromMountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Unmounting old persistent disk")
	}

	err = p.diskManager.GetMounter().Remount(toMountPoint, fromMountPoint)
	if err != nil {
		err = bosherr.WrapError(err, "Remounting new disk on original mountpoint")
	}

	if p.options.DevicePathResolutionType == "iscsi" && iscsiID != "" {
		err = p.flushMultipathDevice(iscsiID)
		if err != nil {
			return err
		}
	}

	return err
}

func (p linux) IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (bool, error) {
	p.logger.Debug(logTag, "Checking whether persistent disk %+v is mounted", diskSettings)
	realPath, timedOut, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if timedOut {
		p.logger.Debug(logTag, "Timed out resolving device path for %+v, ignoring", diskSettings)
		return false, nil
	}
	if err != nil {
		return false, bosherr.WrapError(err, "Getting real device path")
	}

	if !p.options.UsePreformattedPersistentDisk {
		realPath = p.partitionPath(realPath, 1)
	}

	return p.diskManager.GetMounter().IsMounted(realPath)
}

func (p linux) StartMonit() error {
	err := p.GetServiceManager().Setup("monit")
	if err != nil {
		return bosherr.WrapError(err, "Setting up monit")
	}

	err = p.monitRetryStrategy.Try()
	if err != nil {
		return bosherr.WrapError(err, "Retrying to start monit")
	}

	return nil
}

func (p linux) SetupMonitUser() error {
	monitUserFilePath := path.Join(p.dirProvider.BaseDir(), "monit", "monit.user")
	err := p.fs.WriteFileString(monitUserFilePath, "vcap:random-password")
	if err != nil {
		return bosherr.WrapError(err, "Writing monit user file")
	}

	return nil
}

func (p linux) GetMonitCredentials() (username, password string, err error) {
	monitUserFilePath := path.Join(p.dirProvider.BaseDir(), "monit", "monit.user")
	credContent, err := p.fs.ReadFileString(monitUserFilePath)
	if err != nil {
		err = bosherr.WrapError(err, "Reading monit user file")
		return
	}

	credParts := strings.SplitN(credContent, ":", 2)
	if len(credParts) != 2 {
		err = bosherr.Error("Malformated monit user file, expecting username and password separated by ':'")
		return
	}

	username = credParts[0]
	password = credParts[1]
	return
}

func (p linux) PrepareForNetworkingChange() error {
	err := p.fs.RemoveAll("/etc/udev/rules.d/70-persistent-net.rules")
	if err != nil {
		return bosherr.WrapError(err, "Removing network rules file")
	}

	return nil
}

func (p linux) DeleteARPEntryWithIP(ip string) error {
	_, _, _, err := p.cmdRunner.RunCommand("ip", "neigh", "flush", "to", ip)
	if err != nil {
		return bosherr.WrapError(err, "Deleting arp entry")
	}

	return nil
}

func (p linux) GetDefaultNetwork(ipProtocol boship.IPProtocol) (boshsettings.Network, error) {
	return p.defaultNetworkResolver.GetDefaultNetwork(ipProtocol)
}

func (p linux) calculateEphemeralDiskPartitionSizes(diskSizeInBytes uint64, desiredSwapSizeInBytes *uint64) (uint64, uint64, error) {
	memStats, err := p.collector.GetMemStats()
	if err != nil {
		return uint64(0), uint64(0), bosherr.WrapError(err, "Getting mem stats")
	}

	totalMemInBytes := memStats.Total

	var swapSizeInBytes uint64

	if desiredSwapSizeInBytes == nil {
		if totalMemInBytes > diskSizeInBytes/2 {
			swapSizeInBytes = diskSizeInBytes / 2
		} else {
			swapSizeInBytes = totalMemInBytes
		}
	} else {
		swapSizeInBytes = *desiredSwapSizeInBytes
	}

	linuxSizeInBytes := diskSizeInBytes - swapSizeInBytes
	return swapSizeInBytes, linuxSizeInBytes, nil
}

func (p linux) findRootDevicePathAndNumber() (string, int, error) {
	mounts, err := p.diskManager.GetMountsSearcher().SearchMounts()
	if err != nil {
		return "", 0, bosherr.WrapError(err, "Searching mounts")
	}

	for _, mount := range mounts {
		if mount.IsRoot() {
			p.logger.Debug(logTag, "Found root partition: `%s'", mount.PartitionPath)

			stdout, _, _, err := p.cmdRunner.RunCommand("readlink", "-f", mount.PartitionPath)
			if err != nil {
				return "", 0, bosherr.WrapError(err, "Shelling out to readlink")
			}
			rootPartition := strings.Trim(stdout, "\n")
			p.logger.Debug(logTag, "Symlink is: `%s'", rootPartition)

			validNVMeRootPartition := regexp.MustCompile(`^(/dev/[a-z]+\d+n\d+)p(\d+)$`)
			validSCSIRootPartition := regexp.MustCompile(`^/dev/[a-z]+\d$`)

			nvmeMatches := validNVMeRootPartition.FindStringSubmatch(rootPartition)
			isValidSCSIPath := validSCSIRootPartition.MatchString(rootPartition)
			if nvmeMatches == nil && !isValidSCSIPath {
				return "", 0, bosherr.Errorf("Root partition has an invalid name%s", rootPartition)
			}

			var devPath string
			var devNum int
			if nvmeMatches != nil {
				devPath = nvmeMatches[1]
				devNum, err = strconv.Atoi(nvmeMatches[2])
				if err != nil {
					return "", 0, bosherr.WrapError(err, "Parsing NVMe partition number failed")
				}
			} else {
				devPath = rootPartition[:len(rootPartition)-1]
				devNum, err = strconv.Atoi(rootPartition[len(rootPartition)-1:])
				if err != nil {
					return "", 0, bosherr.WrapError(err, "Parsing device number failed")
				}
			}

			return devPath, devNum, nil
		}
	}
	return "", 0, bosherr.Error("Getting root partition device")
}

func (p linux) createEphemeralPartitionsOnRootDevice(desiredSwapSizeInBytes *uint64, labelPrefix string) (string, string, error) {
	p.logger.Info(logTag, "Creating swap & ephemeral partitions on root disk...")
	p.logger.Debug(logTag, "Determining root device")

	rootDevicePath, rootDeviceNumber, err := p.findRootDevicePathAndNumber()
	if err != nil {
		return "", "", bosherr.WrapError(err, "Finding root partition device")
	}
	p.logger.Debug(logTag, "Found root device `%s'", rootDevicePath)

	p.logger.Debug(logTag, "Getting remaining size of `%s'", rootDevicePath)
	remainingSizeInBytes, err := p.diskManager.GetRootDevicePartitioner().GetDeviceSizeInBytes(rootDevicePath)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Getting root device remaining size")
	}

	if remainingSizeInBytes < minRootEphemeralSpaceInBytes {
		return "", "", newInsufficientSpaceError(remainingSizeInBytes, minRootEphemeralSpaceInBytes)
	}

	swapPartitionPath, dataPartitionPath, err := p.partitionDisk(remainingSizeInBytes, desiredSwapSizeInBytes, rootDevicePath, rootDeviceNumber+1, p.diskManager.GetRootDevicePartitioner(), labelPrefix)

	if err != nil {
		return "", "", bosherr.WrapErrorf(err, "Partitioning root device `%s'", rootDevicePath)
	}

	return swapPartitionPath, dataPartitionPath, nil
}

func (p linux) partitionEphemeralDisk(realPath string, desiredSwapSizeInBytes *uint64, labelPrefix string) (string, string, error) {
	p.logger.Info(logTag, "Creating swap & ephemeral partitions on ephemeral disk...")
	p.logger.Debug(logTag, "Getting device size of `%s'", realPath)
	diskSizeInBytes, err := p.diskManager.GetEphemeralDevicePartitioner().GetDeviceSizeInBytes(realPath)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Getting device size")
	}

	swapPartitionPath, dataPartitionPath, err := p.partitionDisk(diskSizeInBytes, desiredSwapSizeInBytes, realPath, 1, p.diskManager.GetEphemeralDevicePartitioner(), labelPrefix)
	if err != nil {
		return "", "", bosherr.WrapErrorf(err, "Partitioning ephemeral disk '%s'", realPath)
	}

	return swapPartitionPath, dataPartitionPath, nil
}

func (p linux) partitionDisk(availableSize uint64, desiredSwapSizeInBytes *uint64, partitionPath string, partitionStartCount int, partitioner boshdisk.Partitioner, labelPrefix string) (string, string, error) {
	p.logger.Debug(logTag, "Calculating partition sizes of `%s', with available size %dB", partitionPath, availableSize)

	swapSizeInBytes, linuxSizeInBytes, err := p.calculateEphemeralDiskPartitionSizes(availableSize, desiredSwapSizeInBytes)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Calculating partition sizes")
	}

	var partitions []boshdisk.Partition
	var swapPartitionPath string
	var dataPartitionPath string

	labelPrefix = prepareDiskLabelPrefix(labelPrefix)
	if swapSizeInBytes == 0 {
		partitions = []boshdisk.Partition{
			{NamePrefix: labelPrefix, SizeInBytes: linuxSizeInBytes, Type: boshdisk.PartitionTypeLinux},
		}
		swapPartitionPath = ""
		dataPartitionPath = p.partitionPath(partitionPath, partitionStartCount)
	} else {
		partitions = []boshdisk.Partition{
			{NamePrefix: labelPrefix, SizeInBytes: swapSizeInBytes, Type: boshdisk.PartitionTypeSwap},
			{NamePrefix: labelPrefix, SizeInBytes: linuxSizeInBytes, Type: boshdisk.PartitionTypeLinux},
		}
		swapPartitionPath = p.partitionPath(partitionPath, partitionStartCount)
		dataPartitionPath = p.partitionPath(partitionPath, partitionStartCount+1)
	}

	p.logger.Info(logTag, "Partitioning `%s' with %s", partitionPath, partitions)
	err = partitioner.Partition(partitionPath, partitions)

	return swapPartitionPath, dataPartitionPath, err
}

func (p linux) RemoveDevTools(packageFileListPath string) error {
	content, err := p.fs.ReadFileString(packageFileListPath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Unable to read Development Tools list file: %s", packageFileListPath)
	}
	content = strings.TrimSpace(content)
	pkgFileList := strings.Split(content, "\n")

	for _, pkgFile := range pkgFileList {
		_, _, _, err = p.cmdRunner.RunCommand("rm", "-rf", pkgFile)
		if err != nil {
			return bosherr.WrapErrorf(err, "Unable to remove package file: %s", pkgFile)
		}
	}

	return nil
}

func (p linux) RemoveStaticLibraries(staticLibrariesListFilePath string) error {
	content, err := p.fs.ReadFileString(staticLibrariesListFilePath)
	if err != nil {
		return bosherr.WrapErrorf(err, "Unable to read static libraries list file: %s", staticLibrariesListFilePath)
	}
	content = strings.TrimSpace(content)
	librariesList := strings.Split(content, "\n")

	for _, library := range librariesList {
		_, _, _, err = p.cmdRunner.RunCommand("rm", "-rf", library)
		if err != nil {
			return bosherr.WrapErrorf(err, "Unable to remove static library: %s", library)
		}
	}

	return nil
}

func (p linux) partitionPath(devicePath string, partitionNumber int) string {
	switch {
	case strings.HasPrefix(devicePath, "/dev/nvme"):
		return fmt.Sprintf("%sp%s", devicePath, strconv.Itoa(partitionNumber))
	case strings.HasPrefix(devicePath, "/dev/mapper/"):
		return fmt.Sprintf("%s-part%s", devicePath, strconv.Itoa(partitionNumber))
	default:
		return fmt.Sprintf("%s%s", devicePath, strconv.Itoa(partitionNumber))
	}
}

func (p linux) generateDefaultEtcHosts(hostname string) (*bytes.Buffer, error) {
	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("etc-hosts").Parse(EtcHostsTemplate))

	err := t.Execute(buffer, hostname)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}

func (p linux) flushMultipathDevice(id string) error {
	p.logger.Debug(logTag, "Flush multipath device: %s", id)
	result, _, _, err := p.cmdRunner.RunCommand("multipath", "-ll")
	if err != nil {
		return bosherr.WrapErrorf(err, "Get multipath information")
	}

	if strings.Contains(result, id) {
		_, _, _, err := p.cmdRunner.RunCommand("multipath", "-f", id)
		if err != nil {
			return bosherr.WrapErrorf(err, "Flush multipath device")
		}
	}

	return nil
}

type insufficientSpaceError struct {
	spaceFound    uint64
	spaceRequired uint64
}

func newInsufficientSpaceError(spaceFound, spaceRequired uint64) insufficientSpaceError {
	return insufficientSpaceError{
		spaceFound:    spaceFound,
		spaceRequired: spaceRequired,
	}
}

func (i insufficientSpaceError) Error() string {
	return fmt.Sprintf("Insufficient remaining disk space (%dB) for ephemeral partition (min: %dB)", i.spaceFound, i.spaceRequired)
}

func (p linux) Shutdown() error {
	_, _, _, err := p.cmdRunner.RunCommand("shutdown", "-P", "0")
	if err != nil {
		return bosherr.WrapErrorf(err, "Failed to shutdown")
	}

	return nil
}

func resolveCanonicalLink(cmdRunner boshsys.CmdRunner, path string) (string, error) {
	stdout, _, _, err := cmdRunner.RunCommand("readlink", "-f", path)
	if err != nil {
		return "", bosherr.WrapError(err, "Shelling out to readlink")
	}

	return strings.Trim(stdout, "\n"), nil
}

func prepareDiskLabelPrefix(labelPrefix string) string {
	// Keep 36 chars to avoid too long GPT partition names
	labelPrefix = "bosh-partition-" + labelPrefix
	if len(labelPrefix) > 33 {
		// Remain one dash and two digits space
		labelPrefix = labelPrefix[0:32]
	}

	return labelPrefix
}
