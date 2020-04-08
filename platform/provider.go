package platform

import (
	"time"

	"code.cloudfoundry.org/clock"
	"github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"

	boshcdrom "github.com/cloudfoundry/bosh-agent/platform/cdrom"
	boshcert "github.com/cloudfoundry/bosh-agent/platform/cert"
	boshdisk "github.com/cloudfoundry/bosh-agent/platform/disk"
	boshnet "github.com/cloudfoundry/bosh-agent/platform/net"
	bosharp "github.com/cloudfoundry/bosh-agent/platform/net/arp"
	boship "github.com/cloudfoundry/bosh-agent/platform/net/ip"
	boshiscsi "github.com/cloudfoundry/bosh-agent/platform/openiscsi"
	boshstats "github.com/cloudfoundry/bosh-agent/platform/stats"
	boshudev "github.com/cloudfoundry/bosh-agent/platform/udevdevice"
	boshvitals "github.com/cloudfoundry/bosh-agent/platform/vitals"
	boshwindisk "github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	boshdirs "github.com/cloudfoundry/bosh-agent/settings/directories"
	bosherror "github.com/cloudfoundry/bosh-utils/errors"
	boshcmd "github.com/cloudfoundry/bosh-utils/fileutil"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshretry "github.com/cloudfoundry/bosh-utils/retrystrategy"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
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

	vitalsService := boshvitals.NewService(statsCollector, dirProvider)

	ipResolver := boship.NewResolver(boship.NetworkInterfaceToAddrsFunc)

	arping := bosharp.NewArping(runner, fs, logger, ArpIterations, ArpIterationDelay, ArpInterfaceCheckDelay)
	interfaceConfigurationCreator := boshnet.NewInterfaceConfigurationCreator(logger)

	interfaceAddressesProvider := boship.NewSystemInterfaceAddressesProvider()
	dnsValidator := boshnet.NewDNSValidator(fs)
	kernelIPv6 := boshnet.NewKernelIPv6Impl(fs, runner, logger)
	macAddressDetector := boshnet.NewMacAddressDetector(fs)

	ubuntuNetManager := boshnet.NewUbuntuNetManager(fs, runner, ipResolver, macAddressDetector, interfaceConfigurationCreator, interfaceAddressesProvider, dnsValidator, arping, kernelIPv6, logger)

	windowsNetManager := boshnet.NewWindowsNetManager(
		runner,
		interfaceConfigurationCreator,
		boshnet.NewMacAddressDetector(nil),
		logger,
		clock,
		fs,
		dirProvider,
	)

	ubuntuCertManager := boshcert.NewUbuntuCertManager(fs, runner, 60, logger)
	windowsCertManager := boshcert.NewWindowsCertManager(fs, runner, dirProvider, logger)

	interfaceManager := boshnet.NewInterfaceManager()

	routesSearcher := boshnet.NewRoutesSearcher(logger, runner, interfaceManager)
	defaultNetworkResolver := boshnet.NewDefaultNetworkResolver(routesSearcher, ipResolver)

	monitRetryable := NewMonitRetryable(runner)
	monitRetryStrategy := boshretry.NewAttemptRetryStrategy(10, 1*time.Second, monitRetryable, logger)

	var devicePathResolver devicepathresolver.DevicePathResolver
	switch options.Linux.DevicePathResolutionType {
	case "virtio":
		udev := boshudev.NewConcreteUdevDevice(runner, logger)
		idDevicePathResolver := devicepathresolver.NewIDDevicePathResolver(500*time.Millisecond, udev, fs)
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
		)
	}

	return provider{
		platforms: map[string]func() Platform{
			"ubuntu":  ubuntu,
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
