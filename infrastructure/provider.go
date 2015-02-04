package infrastructure

import (
	"time"

	boshdpresolv "github.com/cloudfoundry/bosh-agent/infrastructure/devicepathresolver"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplatform "github.com/cloudfoundry/bosh-agent/platform"
	boshudev "github.com/cloudfoundry/bosh-agent/platform/udevdevice"
)

type Provider struct {
	platform boshplatform.Platform
	options  ProviderOptions
	logger   boshlog.Logger
}

type ProviderOptions struct {
	// e.g. possible values: vsphere, mapped, ''
	DevicePathResolutionType string

	// e.g. possible values: dhcp, manual, ''
	NetworkingType string

	StaticEphemeralDiskPath string

	Settings SettingsOptions
}

func NewProvider(
	platform boshplatform.Platform,
	options ProviderOptions,
	logger boshlog.Logger,
) Provider {
	return Provider{
		platform: platform,
		options:  options,
		logger:   logger,
	}
}

func (p Provider) Get() (Infrastructure, error) {
	fs := p.platform.GetFs()

	settingsSourceFactory := NewSettingsSourceFactory(p.options.Settings, fs, p.platform, p.logger)

	settingsSource, err := settingsSourceFactory.New()
	if err != nil {
		return nil, err
	}

	udev := boshudev.NewConcreteUdevDevice(p.platform.GetRunner(), p.logger)
	idDevicePathResolver := boshdpresolv.NewIDDevicePathResolver(500*time.Millisecond, udev, fs)
	mappedDevicePathResolver := boshdpresolv.NewMappedDevicePathResolver(500*time.Millisecond, fs)

	devicePathResolvers := map[string]boshdpresolv.DevicePathResolver{
		"virtio": boshdpresolv.NewVirtioDevicePathResolver(idDevicePathResolver, mappedDevicePathResolver, p.logger),
		"scsi":   boshdpresolv.NewScsiDevicePathResolver(500*time.Millisecond, fs),
	}

	defaultDevicePathResolver := boshdpresolv.NewIdentityDevicePathResolver()

	inf := NewGenericInfrastructure(
		p.platform,
		settingsSource,
		devicePathResolvers,
		defaultDevicePathResolver,

		p.options.DevicePathResolutionType,
		p.options.NetworkingType,
		p.options.StaticEphemeralDiskPath,

		p.logger,
	)

	return inf, nil
}
