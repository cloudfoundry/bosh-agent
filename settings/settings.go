package settings

import (
	"fmt"
	"net"
	"strconv"

	"github.com/cloudfoundry/bosh-agent/v2/platform/disk"
)

type DiskAssociations []DiskAssociation

type DiskAssociation struct {
	Name    string `json:"name"`
	DiskCID string `json:"cid"`
}

const (
	RootUsername        = "root"
	VCAPUsername        = "vcap"
	AdminGroup          = "admin"
	SudoersGroup        = "bosh_sudoers"
	SshersGroup         = "bosh_sshers"
	EphemeralUserPrefix = "bosh_"
)

type Settings struct {
	AgentID        string         `json:"agent_id"`
	Blobstore      Blobstore      `json:"blobstore"`
	Disks          Disks          `json:"disks"`
	Env            Env            `json:"env"`
	Networks       Networks       `json:"networks"`
	NTP            []string       `json:"ntp"`
	Mbus           string         `json:"mbus"`
	VM             VM             `json:"vm"`
	UpdateSettings UpdateSettings `json:"-"`
}

type Source interface {
	PublicSSHKeyForUsername(string) (string, error)
	Settings() (Settings, error)
}

type Blobstore struct {
	Type    string                 `json:"provider"`
	Options map[string]interface{} `json:"options"`
}

type Disks struct {
	// e.g "/dev/sda", "1"
	System string `json:"system"`

	// Older CPIs returned disk settings as string
	// e.g "/dev/sdb", "2"
	// Newer CPIs will populate it in a hash
	// e.g {"path" => "/dev/sdc", "volume_id" => "3"}
	//     {"lun" => "0", "host_device_id" => "{host-device-id}"}
	Ephemeral interface{} `json:"ephemeral"`

	// Older CPIs returned disk settings as strings
	// e.g {"disk-3845-43758-7243-38754" => "/dev/sdc"}
	//     {"disk-3845-43758-7243-38754" => "3"}
	// Newer CPIs will populate it in a hash:
	// e.g {"disk-3845-43758-7243-38754" => {"path" => "/dev/sdc"}}
	//     {"disk-3845-43758-7243-38754" => {"volume_id" => "3"}}
	//     {"disk-3845-43758-7243-38754" => {"lun" => "0", "host_device_id" => "{host-device-id}"}}
	Persistent map[string]interface{} `json:"persistent"`

	RawEphemeral []DiskSettings `json:"raw_ephemeral"`
}

type DiskSettings struct {
	ID           string
	DeviceID     string
	VolumeID     string
	Lun          string
	HostDeviceID string
	Path         string

	// iscsi related
	ISCSISettings ISCSISettings

	FileSystemType disk.FileSystemType
	MountOptions   []string

	Partitioner string
}

type ISCSISettings struct {
	InitiatorName string
	Username      string
	Target        string
	Password      string
}

type VM struct {
	Name string `json:"name"`
}

func (s Settings) TmpFSEnabled() bool {
	return s.Env.Bosh.Agent.Settings.TmpFS || s.Env.Bosh.JobDir.TmpFS
}

func (s Settings) PersistentDiskSettings(diskID string) (DiskSettings, bool) {
	for key, settings := range s.Disks.Persistent {
		if key == diskID {
			return s.populatePersistentDiskSettings(diskID, settings), true
		}
	}

	return DiskSettings{}, false
}

func (s Settings) PersistentDiskSettingsFromHint(diskID string, diskHint interface{}) DiskSettings {
	return s.populatePersistentDiskSettings(diskID, diskHint)
}

func (s Settings) EphemeralDiskSettings() DiskSettings {
	diskSettings := DiskSettings{}

	if s.Disks.Ephemeral != nil { //nolint:nestif
		if hashSettings, ok := s.Disks.Ephemeral.(map[string]interface{}); ok {
			if path, ok := hashSettings["path"]; ok {
				diskSettings.Path = path.(string)
			}
			if volumeID, ok := hashSettings["volume_id"]; ok {
				diskSettings.VolumeID = volumeID.(string)
			}
			if deviceID, ok := hashSettings["id"]; ok {
				diskSettings.DeviceID = deviceID.(string)
			}
			if lun, ok := hashSettings["lun"]; ok {
				diskSettings.Lun = lun.(string)
			}
			if hostDeviceID, ok := hashSettings["host_device_id"]; ok {
				diskSettings.HostDeviceID = hostDeviceID.(string)
			}
		} else if stringSetting, ok := s.Disks.Ephemeral.(string); ok {
			// Old CPIs return disk path (string) or volume id (string) as disk settings
			diskSettings.Path = stringSetting
			diskSettings.VolumeID = stringSetting
		}
	}

	return diskSettings
}

func (s Settings) RawEphemeralDiskSettings() (devices []DiskSettings) {
	return s.Disks.RawEphemeral
}

func (s Settings) GetMbusURL() string {
	if len(s.UpdateSettings.Mbus.URLs) > 0 {
		return s.UpdateSettings.Mbus.URLs[0]
	}
	if len(s.Env.Bosh.Mbus.URLs) > 0 {
		return s.Env.Bosh.Mbus.URLs[0]
	}

	return s.Mbus
}

func (s Settings) GetMbusCerts() CertKeyPair {
	if s.UpdateSettings.Mbus.Cert.CA != "" {
		return s.UpdateSettings.Mbus.Cert
	}
	return s.Env.Bosh.Mbus.Cert
}

func (s Settings) GetBlobstore() Blobstore {
	if len(s.UpdateSettings.Blobstores) > 0 {
		return s.UpdateSettings.Blobstores[0]
	}
	if len(s.Env.Bosh.Blobstores) > 0 {
		return s.Env.Bosh.Blobstores[0]
	}
	return s.Blobstore
}

func (s Settings) GetNtpServers() []string {
	if s.Env.Bosh.NTP != nil {
		return s.Env.Bosh.NTP
	}

	return s.NTP
}

func (s Settings) populatePersistentDiskSettings(diskID string, settingsInfo interface{}) DiskSettings {
	diskSettings := DiskSettings{
		ID: diskID,
	}

	if hashSettings, ok := settingsInfo.(map[string]interface{}); ok { //nolint:nestif
		if path, ok := hashSettings["path"]; ok {
			diskSettings.Path = path.(string)
		}
		if volumeID, ok := hashSettings["volume_id"]; ok {
			diskSettings.VolumeID = volumeID.(string)
		}
		if deviceID, ok := hashSettings["id"]; ok {
			diskSettings.DeviceID = deviceID.(string)
		}
		if lun, ok := hashSettings["lun"]; ok {
			diskSettings.Lun = lun.(string)
		}
		if hostDeviceID, ok := hashSettings["host_device_id"]; ok {
			diskSettings.HostDeviceID = hostDeviceID.(string)
		}
		if iSCSISettings, ok := hashSettings["iscsi_settings"]; ok {
			if hashISCSISettings, ok := iSCSISettings.(map[string]interface{}); ok {
				if username, ok := hashISCSISettings["username"]; ok {
					diskSettings.ISCSISettings.Username = username.(string)
				}
				if password, ok := hashISCSISettings["password"]; ok {
					diskSettings.ISCSISettings.Password = password.(string)
				}
				if initiator, ok := hashISCSISettings["initiator_name"]; ok {
					diskSettings.ISCSISettings.InitiatorName = initiator.(string)
				}
				if target, ok := hashISCSISettings["target"]; ok {
					diskSettings.ISCSISettings.Target = target.(string)
				}
			}
		}
	} else if stringSetting, ok := settingsInfo.(string); ok {
		// Old CPIs return disk path (string) or volume id (string) as disk settings
		diskSettings.Path = stringSetting
		diskSettings.VolumeID = stringSetting
	}

	diskSettings.FileSystemType = s.Env.PersistentDiskFS
	diskSettings.MountOptions = s.Env.PersistentDiskMountOptions
	diskSettings.Partitioner = s.Env.PersistentDiskPartitioner

	return diskSettings
}

type Env struct {
	Bosh                       BoshEnv             `json:"bosh"`
	PersistentDiskFS           disk.FileSystemType `json:"persistent_disk_fs"`
	PersistentDiskMountOptions []string            `json:"persistent_disk_mount_options"`
	PersistentDiskPartitioner  string              `json:"persistent_disk_partitioner"`
}

func (e Env) GetPassword() string {
	return e.Bosh.Password
}

func (e Env) GetKeepRootPassword() bool {
	return e.Bosh.KeepRootPassword
}

func (e Env) GetRemoveDevTools() bool {
	return e.Bosh.RemoveDevTools
}

func (e Env) GetRemoveStaticLibraries() bool {
	return e.Bosh.RemoveStaticLibraries
}

func (e Env) GetAuthorizedKeys() []string {
	return e.Bosh.AuthorizedKeys
}

func (e Env) GetSwapSizeInBytes() *uint64 {
	if e.Bosh.SwapSizeInMB == nil {
		return nil
	}

	result := *e.Bosh.SwapSizeInMB * 1024 * 1024
	return &result
}

func (e Env) GetParallel() *int {
	result := 5
	if e.Bosh.Parallel != nil {
		result = *e.Bosh.Parallel
	}
	return &result
}

type BoshEnv struct {
	Agent                 AgentEnv    `json:"agent"`
	Password              string      `json:"password"`
	KeepRootPassword      bool        `json:"keep_root_password"`
	RemoveDevTools        bool        `json:"remove_dev_tools"`
	RemoveStaticLibraries bool        `json:"remove_static_libraries"`
	AuthorizedKeys        []string    `json:"authorized_keys"`
	SwapSizeInMB          *uint64     `json:"swap_size"`
	Mbus                  MBus        `json:"mbus"`
	IPv6                  IPv6        `json:"ipv6"`
	JobDir                JobDir      `json:"job_dir"`
	RunDir                RunDir      `json:"run_dir"`
	Blobstores            []Blobstore `json:"blobstores"`
	NTP                   []string    `json:"ntp,omitempty"`
	Parallel              *int        `json:"parallel"`
}

type AgentEnv struct {
	Settings AgentSettings `json:"settings"`
}

type AgentSettings struct {
	TmpFS bool `json:"tmpfs"`
}

type MBus struct {
	Cert CertKeyPair `json:"cert"`
	URLs []string    `json:"urls"`
}

type CertKeyPair struct {
	CA          string `json:"ca"`
	PrivateKey  string `json:"private_key"`
	Certificate string `json:"certificate"`
}

type IPv6 struct {
	Enable bool `json:"enable"`
}

type JobDir struct {
	TmpFS bool `json:"tmpfs"`

	// Passed to mount directly
	TmpFSSize string `json:"tmpfs_size"`
}

type RunDir struct {
	// Passed to mount directly
	TmpFSSize string `json:"tmpfs_size"`
}

type DNSRecords struct {
	Version uint64      `json:"Version"`
	Records [][2]string `json:"records"`
}

type NetworkType string

const (
	NetworkTypeDynamic NetworkType = "dynamic"
	NetworkTypeVIP     NetworkType = "vip"
)

type Route struct {
	Destination string
	Gateway     string
	Netmask     string
}

type Routes []Route

type Network struct {
	Type NetworkType `json:"type"`

	IP       string `json:"ip"`
	Netmask  string `json:"netmask"`
	Gateway  string `json:"gateway"`
	Prefix   string `json:"prefix"`
	Resolved bool   `json:"resolved"` // was resolved via DHCP
	UseDHCP  bool   `json:"use_dhcp"`

	Default []string `json:"default"`
	DNS     []string `json:"dns"`

	Mac string `json:"mac"`

	Preconfigured bool   `json:"preconfigured"`
	Routes        Routes `json:"routes,omitempty"`

	Alias string `json:"alias,omitempty"`
}

type Networks map[string]Network

func (n Network) IsDefaultFor(category string) bool {
	return stringArrayContains(n.Default, category)
}

func (n Networks) NetworksForMac(mac string) []Network {
	var networks []Network
	for _, network := range n {
		if network.Mac == mac {
			networks = append(networks, network)
		}
	}
	if len(networks) == 0 {
		networks = append(networks, Network{})
	}

	return networks
}

func (n Networks) DefaultNetworkFor(category string) (Network, bool) {
	if len(n) == 1 {
		for _, net := range n {
			return net, true
		}
	}

	for _, net := range n {
		if net.IsDefaultFor(category) {
			return net, true
		}
	}

	return Network{}, false
}

func stringArrayContains(stringArray []string, str string) bool {
	for _, s := range stringArray {
		if s == str {
			return true
		}
	}
	return false
}

func (n Networks) DefaultIP() (ip string, found bool) {
	for _, networkSettings := range n {
		if ip == "" {
			ip = networkSettings.IP
		}
		if len(networkSettings.Default) > 0 {
			ip = networkSettings.IP
		}
	}

	if ip != "" {
		found = true
	}
	return
}

func (n Networks) IPs() (ips []string) {
	for _, net := range n {
		if net.IP != "" {
			ips = append(ips, net.IP)
		}
	}
	return
}

func (n Networks) HasInterfaceAlias() bool {
	for _, network := range n {
		if network.IsVIP() {
			// Skip VIP networks since we do not configure interfaces for them
			continue
		}

		if network.Alias != "" {
			return true
		}
	}

	return false
}

func (n Networks) IsPreconfigured() bool {
	for _, network := range n {
		if network.IsVIP() {
			// Skip VIP networks since we do not configure interfaces for them
			continue
		}

		if !network.Preconfigured {
			return false
		}
	}

	return true
}

func (n Network) String() string {
	return fmt.Sprintf(
		"type: '%s', ip: '%s', prefix: '%s', netmask: '%s', gateway: '%s', mac: '%s', resolved: '%t', preconfigured: '%t', use_dhcp: '%t'",
		n.Type, n.IP, n.Prefix, n.Netmask, n.Gateway, n.Mac, n.Resolved, n.Preconfigured, n.UseDHCP,
	)
}

func (n Network) IsDHCP() bool {
	if n.IsVIP() {
		return false
	}

	if n.isDynamic() {
		return true
	}

	if n.UseDHCP {
		return true
	}

	// If manual network does not have IP and Netmask it cannot be statically
	// configured. We want to keep track how originally the network was resolved.
	// Otherwise it will be considered as static on subsequent checks.
	isStatic := (n.IP != "" && n.Netmask != "")
	return n.Resolved || !isStatic
}

func (n Network) isDynamic() bool {
	return n.Type == NetworkTypeDynamic
}

func (n Network) IsVIP() bool {
	return n.Type == NetworkTypeVIP
}

func NetmaskToCIDR(netmask string, ipv6 bool) (string, error) {
	ip := net.ParseIP(netmask)
	if ipv6 {
		ipv6mask := net.IPMask(ip)
		ones, _ := ipv6mask.Size()
		if ipv6mask.String() != "00000000000000000000000000000000" && ones == 0 {
			return "0", fmt.Errorf("netmask cannot be converted to CIDR: %s", netmask)
		}
		return strconv.Itoa(ones), nil
	}

	ipv4mask := net.IPMask(ip.To4())
	ones, _ := ipv4mask.Size()
	if ipv4mask.String() != "00000000" && ones == 0 {
		return "0", fmt.Errorf("netmask cannot be converted to CIDR: %s", netmask)
	}
	return strconv.Itoa(ones), nil
}

// {
//	"agent_id": "bm-xxxxxxxx",
//	"blobstore": {
//		"options": {
//			"blobstore_path": "/var/vcap/micro_bosh/data/cache"
//		},
//		"provider": "local"
//	},
//	"disks": {
//		"ephemeral": "/dev/sdb",
//		"persistent": {
//			"vol-xxxxxx": "/dev/sdf"
//		},
//		"system": "/dev/sda1"
//	},
//	"env": {
//		"bosh": {
//			"password": null
//			"mbus": {
//				"url": "nats://localhost:ddd",
//				"ca": "....."
//			}
//      },
//      "persistent_disk_fs": "xfs"
//	},
//  "trusted_certs": "very\nlong\nmultiline\nstring"
//	"mbus": "https://vcap:b00tstrap@0.0.0.0:6868",
//	"networks": {
//		"bosh": {
//			"cloud_properties": {
//				"subnet": "subnet-xxxxxx"
//			},
//			"default": [
//				"dns",
//				"gateway"
//			],
//			"dns": [
//				"xx.xx.xx.xx"
//			],
//			"gateway": null,
//			"ip": "xx.xx.xx.xx",
//			"netmask": null,
//			"type": "manual"
//		},
//		"vip": {
//			"cloud_properties": {},
//			"ip": "xx.xx.xx.xx",
//			"type": "vip"
//		}
//	},
//	"ntp": [
//		"0.north-america.pool.ntp.org",
//		"1.north-america.pool.ntp.org",
//		"2.north-america.pool.ntp.org",
//		"3.north-america.pool.ntp.org"
//	],
//	"vm": {
//		"name": "vm-xxxxxxxx"
//	}
// }
