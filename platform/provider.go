package platform

import (
	gonet "net"
	"time"

	"code.cloudfoundry.org/clock"

	bosherror "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	boshlogstarprovider "github.com/cloudfoundry/bosh-agent/v2/agent/logstarprovider"
	"github.com/cloudfoundry/bosh-agent/v2/infrastructure/devicepathresolver"
	boshcdrom "github.com/cloudfoundry/bosh-agent/v2/platform/cdrom"
	boshcert "github.com/cloudfoundry/bosh-agent/v2/platform/cert"
	boshdisk "github.com/cloudfoundry/bosh-agent/v2/platform/disk"
	boshnet "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	bosharp "github.com/cloudfoundry/bosh-agent/v2/platform/net/arp"
	"github.com/cloudfoundry/bosh-agent/v2/platform/net/dnsresolver"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
	boshiscsi "github.com/cloudfoundry/bosh-agent/v2/platform/openiscsi"
	boshstats "github.com/cloudfoundry/bosh-agent/v2/platform/stats"
	boshudev "github.com/cloudfoundry/bosh-agent/v2/platform/udevdevice"
	boshvitals "github.com/cloudfoundry/bosh-agent/v2/platform/vitals"
	boshwindisk "github.com/cloudfoundry/bosh-agent/v2/platform/windows/disk"
	"github.com/cloudfoundry/bosh-agent/v2/servicemanager"
	boshdirs "github.com/cloudfoundry/bosh-agent/v2/settings/directories"
)

const (
	ArpIterations          = 20
	ArpIterationDelay      = 5 * time.Second
	ArpInterfaceCheckDelay = 100 * time.Millisecond
)

const (
	SigarStatsCollectionInterval = 10 * time.Second
)

type Provider interface {
	Get(name string) (Platform, error)
}

type provider struct {
	platforms map[string]func() Platform
}

type Options struct {
	Linux   LinuxOptions
	Windows WindowsOptions
}

func NewProvider(logger boshlog.Logger, dirProvider boshdirs.Provider, statsCollector boshstats.Collector, fs boshsys.FileSystem, options Options, bootstrapState *BootstrapState, clock clock.Clock, auditLogger AuditLogger) Provider {
	runner := boshsys.NewExecCmdRunner(logger)

	diskManagerOpts := boshdisk.LinuxDiskManagerOpts{
		BindMount:       options.Linux.BindMountPersistentDisk,
		PartitionerType: options.Linux.PartitionerType,
	}

	auditLogger.StartLogging()

	linuxDiskManager := boshdisk.NewLinuxDiskManager(logger, runner, fs, diskManagerOpts)
	windowsDiskManager := boshwindisk.NewWindowsDiskManager(runner)
	udev := boshudev.NewConcreteUdevDevice(runner, logger)
	linuxCdrom := boshcdrom.NewLinuxCdrom("/dev/sr0", udev, runner)
	linuxCdutil := boshcdrom.NewCdUtil(dirProvider.SettingsDir(), fs, linuxCdrom, logger)

	compressor := boshcmd.NewTarballCompressor(runner, fs)
	copier := boshcmd.NewGenericCpCopier(fs, logger)

	// Kick of stats collection as soon as possible
	statsCollector.StartCollecting(SigarStatsCollectionInterval, nil)

	vitalsService := boshvitals.NewService(statsCollector, dirProvider, linuxDiskManager.GetMounter())

	ipResolver := boship.NewResolver(boship.NetworkInterfaceToAddrsFunc)

	arping := bosharp.NewArping(runner, fs, logger, ArpIterations, ArpIterationDelay, ArpInterfaceCheckDelay)
	interfaceConfigurationCreator := boshnet.NewInterfaceConfigurationCreator(logger)

	interfaceAddressesProvider := boship.NewSystemInterfaceAddressesProvider()

	var dnsResolver dnsresolver.DNSResolver
	switch options.Linux.ServiceManager {
	case "systemd":
		dnsResolver = dnsresolver.NewSystemdResolver(fs, runner)
	default:
		dnsResolver = dnsresolver.NewResolveConfResolver(fs, runner)
	}

	kernelIPv6 := boshnet.NewKernelIPv6Impl(fs, runner, logger)
	macAddressDetector := boshnet.NewLinuxMacAddressDetector(fs)

	centosNetManager := boshnet.NewCentosNetManager(fs, runner, ipResolver, macAddressDetector, interfaceConfigurationCreator, interfaceAddressesProvider, dnsResolver, arping, logger)
	ubuntuNetManager := boshnet.NewUbuntuNetManager(fs, runner, ipResolver, macAddressDetector, interfaceConfigurationCreator, interfaceAddressesProvider, dnsResolver, arping, kernelIPv6, logger)

	windowsNetManager := boshnet.NewWindowsNetManager(
		runner,
		interfaceConfigurationCreator,
		boshnet.NewWindowsMacAddressDetector(runner, gonet.Interfaces),
		logger,
		clock,
		fs,
		dirProvider,
	)

	centosCertManager := boshcert.NewCentOSCertManager(fs, runner, 0, logger)
	ubuntuCertManager := boshcert.NewUbuntuCertManager(fs, runner, 60, logger)
	windowsCertManager := boshcert.NewWindowsCertManager(fs, runner, dirProvider, logger)

	interfaceManager := boshnet.NewInterfaceManager()

	routesSearcher := boshnet.NewRoutesSearcher(logger, runner, interfaceManager)
	defaultNetworkResolver := boshnet.NewDefaultNetworkResolver(routesSearcher, ipResolver)

	var serviceManager servicemanager.ServiceManager
	if options.Linux.ServiceManager == "systemd" {
		serviceManager = servicemanager.NewSystemdServiceManager(runner)
	} else {
		serviceManager = servicemanager.NewSvServiceManager(fs, runner)
	}

	monitRetryable := NewMonitRetryable(serviceManager)
	monitRetryStrategy := boshretry.NewAttemptRetryStrategy(10, 1*time.Second, monitRetryable, logger)

	var devicePathResolver devicepathresolver.DevicePathResolver
	switch options.Linux.DevicePathResolutionType {
	case "virtio":
		udev := boshudev.NewConcreteUdevDevice(runner, logger)
		idDevicePathResolver := devicepathresolver.NewIDDevicePathResolver(500*time.Millisecond, udev, fs, options.Linux.StripVolumeRegex)
		mappedDevicePathResolver := devicepathresolver.NewMappedDevicePathResolver(30000*time.Millisecond, fs)
		devicePathResolver = devicepathresolver.NewVirtioDevicePathResolver(idDevicePathResolver, mappedDevicePathResolver, logger)
	case "scsi":
		scsiIDPathResolver := devicepathresolver.NewSCSIIDDevicePathResolver(50000*time.Millisecond, fs, logger)
		scsiVolumeIDPathResolver := devicepathresolver.NewSCSIVolumeIDDevicePathResolver(500*time.Millisecond, fs)
		scsiLunPathResolver := devicepathresolver.NewSCSILunDevicePathResolver(50000*time.Millisecond, fs, logger)
		devicePathResolver = devicepathresolver.NewScsiDevicePathResolver(scsiVolumeIDPathResolver, scsiIDPathResolver, scsiLunPathResolver)
	case "iscsi":
		identityPathResolver := devicepathresolver.NewIdentityDevicePathResolver()
		iscsiAdm := boshiscsi.NewConcreteOpenIscsiAdmin(fs, runner, logger)
		iscsiPathResolver := devicepathresolver.NewIscsiDevicePathResolver(50000*time.Millisecond, runner, iscsiAdm, fs, dirProvider, logger)
		devicePathResolver = devicepathresolver.NewMultipathDevicePathResolver(identityPathResolver, iscsiPathResolver, logger)

	default:
		devicePathResolver = devicepathresolver.NewIdentityDevicePathResolver()
	}

	uuidGenerator := boshuuid.NewGenerator()
	logsTarProvider := boshlogstarprovider.NewLogsTarProvider(compressor, copier, dirProvider)

	var centos = func() Platform {
		return NewLinuxPlatform(
			fs,
			runner,
			statsCollector,
			compressor,
			copier,
			dirProvider,
			vitalsService,
			linuxCdutil,
			linuxDiskManager,
			centosNetManager,
			centosCertManager,
			monitRetryStrategy,
			devicePathResolver,
			bootstrapState,
			options.Linux,
			logger,
			defaultNetworkResolver,
			uuidGenerator,
			auditLogger,
			logsTarProvider,
			serviceManager,
		)
	}

	var ubuntu = func() Platform {
		return NewLinuxPlatform(
			fs,
			runner,
			statsCollector,
			compressor,
			copier,
			dirProvider,
			vitalsService,
			linuxCdutil,
			linuxDiskManager,
			ubuntuNetManager,
			ubuntuCertManager,
			monitRetryStrategy,
			devicePathResolver,
			bootstrapState,
			options.Linux,
			logger,
			defaultNetworkResolver,
			uuidGenerator,
			auditLogger,
			logsTarProvider,
			serviceManager,
		)
	}

	var windows = func() Platform {
		return NewWindowsPlatform(
			statsCollector,
			fs,
			runner,
			dirProvider,
			windowsNetManager,
			windowsCertManager,
			devicePathResolver,
			options,
			logger,
			defaultNetworkResolver,
			auditLogger,
			uuidGenerator,
			windowsDiskManager,
			logsTarProvider,
		)
	}

	var dummy = func() Platform {
		return NewDummyPlatform(
			statsCollector,
			fs,
			runner,
			dirProvider,
			devicePathResolver,
			logger,
			auditLogger,
			logsTarProvider,
		)
	}

	return provider{
		platforms: map[string]func() Platform{
			"ubuntu":  ubuntu,
			"centos":  centos,
			"dummy":   dummy,
			"windows": windows,
		},
	}
}

func (p provider) Get(name string) (Platform, error) {
	plat, found := p.platforms[name]
	if !found {
		return nil, bosherror.Errorf("Platform %s could not be found", name)
	}
	return plat(), nil
}
