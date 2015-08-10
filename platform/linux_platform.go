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
	"time"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	bosherr "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-agent/internal/github.com/cloudfoundry/bosh-utils/system"
	boshcert "github.com/cloudfoundry/bosh-agent/platform/cert"
	boshdevutil "github.com/cloudfoundry/bosh-agent/platform/deviceutil"
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshnet "github.com/cloudfoundry/bosh-agent/platform/net"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshdir "github.com/cloudfoundry/bosh-agent/settings/directories"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
)

const (
	ephemeralDiskPermissions  = os.FileMode(0750)
	persistentDiskPermissions = os.FileMode(0700)

	logDirPermissions      = os.FileMode(0750)
	runDirPermissions      = os.FileMode(0750)
	userBaseDirPermissions = os.FileMode(0755)
	tmpDirPermissions      = os.FileMode(0755) // 0755 to make sure that vcap user can use new temp dir

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

	// Strategy for resolving device paths;
	// possible values: virtio, scsi, ''
	DevicePathResolutionType string
}

type linux struct {
	fs                     boshsys.FileSystem
	cmdRunner              boshsys.CmdRunner
	collector              boshstats.Collector
	compressor             boshcmd.Compressor
	copier                 boshcmd.Copier
	dirProvider            boshdirs.Provider
	vitalsService          boshvitals.Service
	cdutil                 boshdevutil.DeviceUtil
	diskManager            boshdisk.Manager
	netManager             boshnet.Manager
	certManager            boshcert.Manager
	monitRetryStrategy     boshretry.RetryStrategy
	devicePathResolver     boshdpresolv.DevicePathResolver
	diskScanDuration       time.Duration
	options                LinuxOptions
	logger                 boshlog.Logger
	defaultNetworkResolver boshsettings.DefaultNetworkResolver
}

func NewLinuxPlatform(
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	collector boshstats.Collector,
	compressor boshcmd.Compressor,
	copier boshcmd.Copier,
	dirProvider boshdirs.Provider,
	vitalsService boshvitals.Service,
	cdutil boshdevutil.DeviceUtil,
	diskManager boshdisk.Manager,
	netManager boshnet.Manager,
	certManager boshcert.Manager,
	monitRetryStrategy boshretry.RetryStrategy,
	devicePathResolver boshdpresolv.DevicePathResolver,
	diskScanDuration time.Duration,
	options LinuxOptions,
	logger boshlog.Logger,
	defaultNetworkResolver boshsettings.DefaultNetworkResolver,
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
		diskScanDuration:       diskScanDuration,
		options:                options,
		logger:                 logger,
		defaultNetworkResolver: defaultNetworkResolver,
	}
}

const logTag = "linuxPlatform"

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

func (p linux) GetDirProvider() (dirProvider boshdir.Provider) {
	return p.dirProvider
}

func (p linux) GetVitalsService() (service boshvitals.Service) {
	return p.vitalsService
}

func (p linux) GetFileContentsFromCDROM(fileName string) (content []byte, err error) {
	contents, err := p.cdutil.GetFilesContents([]string{fileName})
	if err != nil {
		return []byte{}, err
	}

	return contents[0], nil
}

func (p linux) GetFilesContentsFromDisk(diskPath string, fileNames []string) ([][]byte, error) {
	return p.diskManager.GetDiskUtil(diskPath).GetFilesContents(fileNames)
}

func (p linux) GetDevicePathResolver() (devicePathResolver boshdpresolv.DevicePathResolver) {
	return p.devicePathResolver
}

func (p linux) SetupNetworking(networks boshsettings.Networks) (err error) {
	return p.netManager.SetupNetworking(networks, nil)
}

func (p linux) GetConfiguredNetworkInterfaces() ([]string, error) {
	return p.netManager.GetConfiguredNetworkInterfaces()
}

func (p linux) GetCertManager() boshcert.Manager {
	return p.certManager
}

func (p linux) SetupRuntimeConfiguration() (err error) {
	_, _, _, err = p.cmdRunner.RunCommand("bosh-agent-rc")
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to bosh-agent-rc")
	}
	return
}

func (p linux) CreateUser(username, password, basePath string) error {
	err := p.fs.MkdirAll(basePath, userBaseDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Making user base path")
	}

	args := []string{"-m", "-b", basePath, "-s", "/bin/bash"}

	if password != "" {
		args = append(args, "-p", password)
	}

	args = append(args, username)

	_, _, _, err = p.cmdRunner.RunCommand("useradd", args...)
	if err != nil {
		return bosherr.WrapError(err, "Shelling out to useradd")
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
	_, _, _, err = p.cmdRunner.RunCommand("userdel", "-r", user)
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

func (p linux) SetupSSH(publicKey, username string) error {
	homeDir, err := p.fs.HomeDir(username)
	if err != nil {
		return bosherr.WrapError(err, "Finding home dir for user")
	}

	sshPath := filepath.Join(homeDir, ".ssh")
	err = p.fs.MkdirAll(sshPath, sshDirPermissions)
	if err != nil {
		return bosherr.WrapError(err, "Making ssh directory")
	}
	err = p.fs.Chown(sshPath, username)
	if err != nil {
		return bosherr.WrapError(err, "Chowning ssh directory")
	}

	authKeysPath := filepath.Join(sshPath, "authorized_keys")
	err = p.fs.WriteFileString(authKeysPath, publicKey)
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
	_, _, _, err = p.cmdRunner.RunCommand("usermod", "-p", encryptedPwd, user)
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to usermod")
	}
	return
}

func (p linux) SetupHostname(hostname string) (err error) {
	_, _, _, err = p.cmdRunner.RunCommand("hostname", hostname)
	if err != nil {
		err = bosherr.WrapError(err, "Shelling out to hostname")
		return
	}

	err = p.fs.WriteFileString("/etc/hostname", hostname)
	if err != nil {
		err = bosherr.WrapError(err, "Writing /etc/hostname")
		return
	}

	buffer := bytes.NewBuffer([]byte{})
	t := template.Must(template.New("etc-hosts").Parse(etcHostsTemplate))

	err = t.Execute(buffer, hostname)
	if err != nil {
		err = bosherr.WrapError(err, "Generating config from template")
		return
	}

	err = p.fs.WriteFile("/etc/hosts", buffer.Bytes())
	if err != nil {
		err = bosherr.WrapError(err, "Writing to /etc/hosts")
	}
	return
}

const etcHostsTemplate = `127.0.0.1 localhost {{ . }}

# The following lines are desirable for IPv6 capable hosts
::1 localhost ip6-localhost ip6-loopback {{ . }}
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts
`

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

	err = p.fs.WriteFile(filepath.Join("/etc/logrotate.d", groupName), buffer.Bytes())
	if err != nil {
		err = bosherr.WrapError(err, "Writing to /etc/logrotate.d")
		return
	}

	return
}

// Logrotate config file - /etc/logrotate.d/<group-name>
const etcLogrotateDTemplate = `# Generated by bosh-agent

{{ .BasePath }}/data/sys/log/*.log {{ .BasePath }}/data/sys/log/*/*.log {{ .BasePath }}/data/sys/log/*/*/*.log {
  missingok
  rotate 7
  compress
  delaycompress
  copytruncate
  size={{ .Size }}
}
`

func (p linux) SetTimeWithNtpServers(servers []string) (err error) {
	serversFilePath := filepath.Join(p.dirProvider.BaseDir(), "/bosh/etc/ntpserver")
	if len(servers) == 0 {
		return
	}

	err = p.fs.WriteFileString(serversFilePath, strings.Join(servers, " "))
	if err != nil {
		err = bosherr.WrapErrorf(err, "Writing to %s", serversFilePath)
		return
	}

	// Make a best effort to sync time now but don't error
	_, _, _, _ = p.cmdRunner.RunCommand("ntpdate")
	return
}

func (p linux) SetupEphemeralDiskWithPath(realPath string) error {
	p.logger.Info(logTag, "Setting up ephemeral disk...")
	mountPoint := p.dirProvider.DataDir()

	mountPointGlob := path.Join(mountPoint, "*")
	contents, err := p.fs.Glob(mountPointGlob)
	if err != nil {
		return bosherr.WrapErrorf(err, "Globbing ephemeral disk mount point `%s'", mountPointGlob)
	}

	if contents != nil && len(contents) > 0 {
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

		swapPartitionPath, dataPartitionPath, err = p.createEphemeralPartitionsOnRootDevice()
		if err != nil {
			return bosherr.WrapError(err, "Creating ephemeral partitions on root device")
		}
	} else {
		swapPartitionPath, dataPartitionPath, err = p.partitionEphemeralDisk(realPath)
		if err != nil {
			return bosherr.WrapError(err, "Partitioning ephemeral disk")
		}
	}

	p.logger.Info(logTag, "Formatting `%s' as swap", swapPartitionPath)
	err = p.diskManager.GetFormatter().Format(swapPartitionPath, boshdisk.FileSystemSwap)
	if err != nil {
		return bosherr.WrapError(err, "Formatting swap")
	}

	p.logger.Info(logTag, "Formatting `%s' as ext4", dataPartitionPath)
	err = p.diskManager.GetFormatter().Format(dataPartitionPath, boshdisk.FileSystemExt4)
	if err != nil {
		return bosherr.WrapError(err, "Formatting data partition with ext4")
	}

	p.logger.Info(logTag, "Mounting `%s' as swap", swapPartitionPath)
	err = p.diskManager.GetMounter().SwapOn(swapPartitionPath)
	if err != nil {
		return bosherr.WrapError(err, "Mounting swap")
	}

	p.logger.Info(logTag, "Mounting `%s' at `%s'", dataPartitionPath, mountPoint)
	err = p.diskManager.GetMounter().Mount(dataPartitionPath, mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Mounting data partition")
	}

	return nil
}

func (p linux) SetupDataDir() error {
	dataDir := p.dirProvider.DataDir()

	sysDataDir := filepath.Join(dataDir, "sys")

	logDir := filepath.Join(sysDataDir, "log")
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

	err = p.setupRunDir(sysDataDir)
	if err != nil {
		return err
	}

	sysDir := filepath.Join(filepath.Dir(dataDir), "sys")
	err = p.fs.Symlink(sysDataDir, sysDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Symlinking '%s' to '%s'", sysDir, sysDataDir)
	}

	return nil
}

func (p linux) setupRunDir(sysDir string) error {
	runDir := filepath.Join(sysDir, "run")

	runDirIsMounted, err := p.IsMountPoint(runDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Checking for mount point %s", runDir)
	}

	if !runDirIsMounted {
		err = p.fs.MkdirAll(runDir, runDirPermissions)
		if err != nil {
			return bosherr.WrapErrorf(err, "Making %s dir", runDir)
		}

		err = p.diskManager.GetMounter().Mount("tmpfs", runDir, "-t", "tmpfs", "-o", "size=1m")
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

func (p linux) SetupTmpDir() error {
	systemTmpDir := "/tmp"
	boshTmpDir := p.dirProvider.TmpDir()
	boshRootTmpPath := filepath.Join(p.dirProvider.DataDir(), "root_tmp")

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
	_, _, _, err = p.cmdRunner.RunCommand("chmod", "0700", "/var/tmp")
	if err != nil {
		return bosherr.WrapError(err, "chmod /var/tmp")
	}

	if p.options.UseDefaultTmpDir {
		return nil
	}

	systemTmpDirIsMounted, err := p.IsMountPoint(systemTmpDir)
	if err != nil {
		return bosherr.WrapErrorf(err, "Checking for mount point %s", systemTmpDir)
	}

	if !systemTmpDirIsMounted {
		// If it's not mounted on /tmp, blow it away
		_, _, _, err = p.cmdRunner.RunCommand("truncate", "-s", "128M", boshRootTmpPath)
		if err != nil {
			return bosherr.WrapError(err, "Truncating root tmp dir")
		}

		_, _, _, err = p.cmdRunner.RunCommand("chmod", "0700", boshRootTmpPath)
		if err != nil {
			return bosherr.WrapError(err, "Chmoding root tmp dir")
		}

		_, _, _, err = p.cmdRunner.RunCommand("mke2fs", "-t", "ext4", "-m", "1", "-F", boshRootTmpPath)
		if err != nil {
			return bosherr.WrapError(err, "Creating root tmp dir filesystem")
		}

		err = p.diskManager.GetMounter().Mount(boshRootTmpPath, systemTmpDir, "-t", "ext4", "-o", "loop")
		if err != nil {
			return bosherr.WrapError(err, "Mounting root tmp dir over /tmp")
		}

		// Change permissions for new mount point
		err = p.changeTmpDirPermissions(systemTmpDir)
		if err != nil {
			return err
		}
	}

	return nil
}

func (p linux) changeTmpDirPermissions(path string) error {
	_, _, _, err := p.cmdRunner.RunCommand("chown", "root:vcap", path)
	if err != nil {
		return bosherr.WrapErrorf(err, "chown %s", path)
	}

	_, _, _, err = p.cmdRunner.RunCommand("chmod", "0770", path)
	if err != nil {
		return bosherr.WrapErrorf(err, "chmod %s", path)
	}

	return nil
}

func (p linux) MountPersistentDisk(diskSetting boshsettings.DiskSettings, mountPoint string) error {
	p.logger.Debug(logTag, "Mounting persistent disk %s at %s", diskSetting.Path, mountPoint)

	err := p.fs.MkdirAll(mountPoint, persistentDiskPermissions)
	if err != nil {
		return bosherr.WrapErrorf(err, "Creating directory %s", mountPoint)
	}

	realPath, _, err := p.devicePathResolver.GetRealDevicePath(diskSetting)
	if err != nil {
		return bosherr.WrapError(err, "Getting real device path")
	}

	if !p.options.UsePreformattedPersistentDisk {
		partitions := []boshdisk.Partition{
			{Type: boshdisk.PartitionTypeLinux},
		}

		err = p.diskManager.GetPartitioner().Partition(realPath, partitions)
		if err != nil {
			return bosherr.WrapError(err, "Partitioning disk")
		}

		partitionPath := realPath + "1"

		err = p.diskManager.GetFormatter().Format(partitionPath, boshdisk.FileSystemExt4)
		if err != nil {
			return bosherr.WrapError(err, "Formatting partition with ext4")
		}

		realPath = partitionPath
	}

	err = p.diskManager.GetMounter().Mount(realPath, mountPoint)
	if err != nil {
		return bosherr.WrapError(err, "Mounting partition")
	}

	return nil
}

func (p linux) UnmountPersistentDisk(diskSettings boshsettings.DiskSettings) (bool, error) {
	p.logger.Debug(logTag, "Unmounting persistent disk %s", diskSettings.Path)

	realPath, timedOut, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if timedOut {
		return false, nil
	}
	if err != nil {
		return false, bosherr.WrapError(err, "Getting real device path")
	}

	if !p.options.UsePreformattedPersistentDisk {
		realPath += "1"
	}

	return p.diskManager.GetMounter().Unmount(realPath)
}

func (p linux) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	if len(diskSettings.Path) == 0 {
		return ""
	}

	realPath, _, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if err != nil {
		return ""
	}

	return realPath
}

func (p linux) IsMountPoint(path string) (bool, error) {
	return p.diskManager.GetMounter().IsMountPoint(path)
}

func (p linux) MigratePersistentDisk(fromMountPoint, toMountPoint string) (err error) {
	p.logger.Debug(logTag, "Migrating persistent disk %v to %v", fromMountPoint, toMountPoint)

	err = p.diskManager.GetMounter().RemountAsReadonly(fromMountPoint)
	if err != nil {
		err = bosherr.WrapError(err, "Remounting persistent disk as readonly")
		return
	}

	// Golang does not implement a file copy that would allow us to preserve dates...
	// So we have to shell out to tar to perform the copy instead of delegating to the FileSystem
	tarCopy := fmt.Sprintf("(tar -C %s -cf - .) | (tar -C %s -xpf -)", fromMountPoint, toMountPoint)
	_, _, _, err = p.cmdRunner.RunCommand("sh", "-c", tarCopy)
	if err != nil {
		err = bosherr.WrapError(err, "Copying files from old disk to new disk")
		return
	}

	_, err = p.diskManager.GetMounter().Unmount(fromMountPoint)
	if err != nil {
		err = bosherr.WrapError(err, "Unmounting old persistent disk")
		return
	}

	err = p.diskManager.GetMounter().Remount(toMountPoint, fromMountPoint)
	if err != nil {
		err = bosherr.WrapError(err, "Remounting new disk on original mountpoint")
	}
	return
}

func (p linux) IsPersistentDiskMounted(diskSettings boshsettings.DiskSettings) (bool, error) {
	p.logger.Debug(logTag, "Checking whether persistent disk %s is mounted", diskSettings.Path)
	realPath, timedOut, err := p.devicePathResolver.GetRealDevicePath(diskSettings)
	if timedOut {
		p.logger.Debug(logTag, "Timed out resolving device path %s, ignoring", diskSettings.Path)
		return false, nil
	}
	if err != nil {
		return false, bosherr.WrapError(err, "Getting real device path")
	}

	if !p.options.UsePreformattedPersistentDisk {
		realPath += "1"
	}

	return p.diskManager.GetMounter().IsMounted(realPath)
}

func (p linux) StartMonit() error {
	err := p.fs.Symlink(filepath.Join("/etc", "sv", "monit"), filepath.Join("/etc", "service", "monit"))
	if err != nil {
		return bosherr.WrapError(err, "Symlinking /etc/service/monit to /etc/sv/monit")
	}

	err = p.monitRetryStrategy.Try()
	if err != nil {
		return bosherr.WrapError(err, "Retrying to start monit")
	}

	return nil
}

func (p linux) SetupMonitUser() error {
	monitUserFilePath := filepath.Join(p.dirProvider.BaseDir(), "monit", "monit.user")
	if !p.fs.FileExists(monitUserFilePath) {
		err := p.fs.WriteFileString(monitUserFilePath, "vcap:random-password")
		if err != nil {
			return bosherr.WrapError(err, "Writing monit user file")
		}
	}

	return nil
}

func (p linux) GetMonitCredentials() (username, password string, err error) {
	monitUserFilePath := filepath.Join(p.dirProvider.BaseDir(), "monit", "monit.user")
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

func (p linux) GetDefaultNetwork() (boshsettings.Network, error) {
	return p.defaultNetworkResolver.GetDefaultNetwork()
}

func (p linux) calculateEphemeralDiskPartitionSizes(diskSizeInBytes uint64) (uint64, uint64, error) {
	memStats, err := p.collector.GetMemStats()
	if err != nil {
		return uint64(0), uint64(0), bosherr.WrapError(err, "Getting mem stats")
	}

	totalMemInBytes := memStats.Total

	var swapSizeInBytes uint64
	if totalMemInBytes > diskSizeInBytes/2 {
		swapSizeInBytes = diskSizeInBytes / 2
	} else {
		swapSizeInBytes = totalMemInBytes
	}

	linuxSizeInBytes := diskSizeInBytes - swapSizeInBytes
	return swapSizeInBytes, linuxSizeInBytes, nil
}

func (p linux) findRootDevicePath() (string, uint8, error) {
	mounts, err := p.diskManager.GetMountsSearcher().SearchMounts()
	if err != nil {
		return "", 0, bosherr.WrapError(err, "Searching mounts")
	}

	var devNum uint8
	var devPath string
	validPartition := regexp.MustCompile(`^/dev/[a-z]+\d$`)

	for _, mount := range mounts {
		stdout, _, _, err := p.cmdRunner.RunCommand("readlink", "-f", mount.PartitionPath)
		if err != nil {
			return "", 0, bosherr.WrapError(err, "Shelling out to readlink")
		}
		partition := strings.Trim(stdout, "\n")
		p.logger.Debug(logTag, "Symlink is: `%s'", partition)

		if mount.MountPoint == "/" && strings.HasPrefix(mount.PartitionPath, "/dev/") {
			p.logger.Debug(logTag, "Found root partition: `%s'", mount.PartitionPath)
			if !validPartition.MatchString(partition) {
				return "", 0, bosherr.Error("Root partition has an invalid name" + partition)
			}
			devNum = partition[len(partition)-1] - 48
			devPath = partition[:len(partition)-1]
			return devPath, devNum, nil
		}
	}
	return "", 0, bosherr.Error("Getting root partition device")
}

func (p linux) createEphemeralPartitionsOnRootDevice() (string, string, error) {
	p.logger.Info(logTag, "Creating swap & ephemeral partitions on root disk...")
	p.logger.Debug(logTag, "Determining root device")

	rootDevicePath, rootDeviceNum, err := p.findRootDevicePath()
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

	p.logger.Debug(logTag, "Calculating partition sizes of `%s', remaining size: %dB", rootDevicePath, remainingSizeInBytes)
	swapSizeInBytes, linuxSizeInBytes, err := p.calculateEphemeralDiskPartitionSizes(remainingSizeInBytes)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Calculating partition sizes")
	}

	partitions := []boshdisk.Partition{
		{SizeInBytes: swapSizeInBytes, Type: boshdisk.PartitionTypeSwap},
		{SizeInBytes: linuxSizeInBytes, Type: boshdisk.PartitionTypeLinux},
	}

	for _, partition := range partitions {
		p.logger.Info(logTag, "Partitioning root device `%s': %s", rootDevicePath, partition)
	}

	err = p.diskManager.GetRootDevicePartitioner().Partition(rootDevicePath, partitions)
	if err != nil {
		return "", "", bosherr.WrapErrorf(err, "Partitioning root device `%s'", rootDevicePath)
	}

	swapPartitionPath := rootDevicePath + strconv.Itoa(int(rootDeviceNum+1))
	dataPartitionPath := rootDevicePath + strconv.Itoa(int(rootDeviceNum+2))
	return swapPartitionPath, dataPartitionPath, nil
}

func (p linux) partitionEphemeralDisk(realPath string) (string, string, error) {
	p.logger.Info(logTag, "Creating swap & ephemeral partitions on ephemeral disk...")
	p.logger.Debug(logTag, "Getting device size of `%s'", realPath)
	diskSizeInBytes, err := p.diskManager.GetPartitioner().GetDeviceSizeInBytes(realPath)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Getting device size")
	}

	p.logger.Debug(logTag, "Calculating ephemeral disk partition sizes of `%s' with total disk size %dB", realPath, diskSizeInBytes)
	swapSizeInBytes, linuxSizeInBytes, err := p.calculateEphemeralDiskPartitionSizes(diskSizeInBytes)
	if err != nil {
		return "", "", bosherr.WrapError(err, "Calculating partition sizes")
	}

	partitions := []boshdisk.Partition{
		{SizeInBytes: swapSizeInBytes, Type: boshdisk.PartitionTypeSwap},
		{SizeInBytes: linuxSizeInBytes, Type: boshdisk.PartitionTypeLinux},
	}

	p.logger.Info(logTag, "Partitioning ephemeral disk `%s' with %s", realPath, partitions)
	err = p.diskManager.GetPartitioner().Partition(realPath, partitions)
	if err != nil {
		return "", "", bosherr.WrapErrorf(err, "Partitioning ephemeral disk `%s'", realPath)
	}

	swapPartitionPath := realPath + "1"
	dataPartitionPath := realPath + "2"
	return swapPartitionPath, dataPartitionPath, nil
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
