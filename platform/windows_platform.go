package platform

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"

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
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

// Administrator user name, this currently exists for testing, but may be useful
// if we ever change the Admin user name for security reasons.
var administratorUserName = "Administrator"

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
	certManager            boshcert.Manager
	defaultNetworkResolver boshsettings.DefaultNetworkResolver
	auditLogger            AuditLogger
	uuidGenerator          boshuuid.Generator
}

func NewWindowsPlatform(
	collector boshstats.Collector,
	fs boshsys.FileSystem,
	cmdRunner boshsys.CmdRunner,
	dirProvider boshdirs.Provider,
	netManager boshnet.Manager,
	certManager boshcert.Manager,
	devicePathResolver boshdpresolv.DevicePathResolver,
	logger boshlog.Logger,
	defaultNetworkResolver boshsettings.DefaultNetworkResolver,
	auditLogger AuditLogger,
	uuidGenerator boshuuid.Generator,
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
		defaultNetworkResolver: defaultNetworkResolver,
		auditLogger:            auditLogger,
		uuidGenerator:          uuidGenerator,
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

	// Grant sshd service read access to the authorized_keys file.
	//
	// Do not use the WindowsPlatform.cmdRunner for this - it passes
	// every command through PowerShell, which breaks this command.
	//
	cmd := exec.Command("icacls.exe", authkeysPath, "/grant", "NT SERVICE\\SSHD:(R)")
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Remove authorized_keys file - don't check the error
		p.fs.RemoveAll(authkeysPath)

		return bosherr.WrapErrorf(err, "Setting ACL on authorized_keys file (%s): %s",
			authkeysPath, string(out))
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
	_, stderr, _, err = p.cmdRunner.RunCommand("w32tm", "/config", "/syncfromflags:manual", manualPeerList)
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

func (p WindowsPlatform) SetupEphemeralDiskWithPath(devicePath string, desiredSwapSizeInBytes *uint64) (err error) {
	return
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

func (p WindowsPlatform) GetEphemeralDiskPath(diskSettings boshsettings.DiskSettings) string {
	return ""
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

	sshdir := filepath.Join(drive, "Program Files", "OpenSSH")
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

// checkSSH checks if the sshd and ssh-agent services are installed and running.
//
// The services are installed during stemcell creation, but are disabled.  The
// job windows-utilities-release/enable_ssh job is used to enable ssh.
func checkSSH() error {
	const ERROR_SERVICE_DOES_NOT_EXIST syscall.Errno = 0x424

	const msgFmt = "%s service not running and start type is disabled.  " +
		"To enable ssh on Windows you must run the enable_ssh job from the " +
		"windows-utilities-release."

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("opening service control manager: %s", err)
	}
	defer m.Disconnect()

	sshd, err := m.OpenService("sshd")
	if err != nil {
		if err == ERROR_SERVICE_DOES_NOT_EXIST {
			return errors.New("sshd is not installed")
		}
		return fmt.Errorf("opening service sshd: %s", err)
	}
	defer sshd.Close()

	agent, err := m.OpenService("ssh-agent")
	if err != nil {
		if err == ERROR_SERVICE_DOES_NOT_EXIST {
			return errors.New("ssh-agent is not installed")
		}
		return fmt.Errorf("opening service ssh-agent: %s", err)
	}
	defer agent.Close()

	st, err := sshd.Query()
	if err != nil {
		return fmt.Errorf("querying status of service (sshd): %s", err)
	}
	if st.State != svc.Running {
		if serviceDisabled(sshd) {
			return fmt.Errorf(msgFmt, "sshd")
		}
		return errors.New("sshd service is not running")
	}

	// ssh-agent is a dependency of sshd so it should always
	// be running if sshd is running - check just to make sure.
	st, err = agent.Query()
	if err != nil {
		return fmt.Errorf("querying status of service ssh-agent: %s", err)
	}
	if st.State != svc.Running {
		if serviceDisabled(agent) {
			return fmt.Errorf(msgFmt, "ssh-agent")
		}
		return errors.New("ssh-agent service is not running")
	}

	return nil
}

func SetAdministratorUserName(name string) (previous string) {
	previous = administratorUserName
	administratorUserName = name
	return previous
}

// Make the function called by GetHostPublicKey configurable for testing.
var sshEnabled func() error = checkSSH

// SetSSHEnabled sets the function called by GetHostPublicKey to determine if
// ssh is enabled.
func SetSSHEnabled(new func() error) (previous func() error) {
	previous = sshEnabled
	sshEnabled = new
	return previous
}

var (
	// Export for testing
	UserHomeDirectory    = userHomeDirectory
	RandomPassword       = randomPassword
	ValidWindowsPassword = validPassword
	LocalAccountNames    = localAccountNames

	// Export for test cleanup
	DeleteUserProfile = deleteUserProfile
)
