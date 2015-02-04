package infrastructure

import (
	"encoding/json"

	mapstruc "github.com/mitchellh/mapstructure"

	bosherr "github.com/cloudfoundry/bosh-agent/errors"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsys "github.com/cloudfoundry/bosh-agent/system"
)

type SettingsOptions struct {
	Sources       SourceOptionsSlice
	UseServerName bool
	UseRegistry   bool
}

// SourceOptionsSlice is used for unmarshalling different source types
type SourceOptionsSlice []SourceOptions

type SourceOptions interface {
	sourceOptionsInterface()
}

type HTTPSourceOptions struct {
	URI string
}

func (o HTTPSourceOptions) sourceOptionsInterface() {}

type ConfigDriveSourceOptions struct {
	DiskPaths []string

	MetaDataPath string
	UserDataPath string

	SettingsPath string
}

func (o ConfigDriveSourceOptions) sourceOptionsInterface() {}

type FileSourceOptions struct {
	MetaDataPath string
	UserDataPath string

	SettingsPath string
}

func (o FileSourceOptions) sourceOptionsInterface() {}

type CDROMSourceOptions struct {
	FileName string
}

func (o CDROMSourceOptions) sourceOptionsInterface() {}

type SettingsSourceFactory struct {
	options  SettingsOptions
	fs       boshsys.FileSystem
	platform boshplat.Platform
	logger   boshlog.Logger
}

func NewSettingsSourceFactory(
	options SettingsOptions,
	fs boshsys.FileSystem,
	platform boshplat.Platform,
	logger boshlog.Logger,
) SettingsSourceFactory {
	return SettingsSourceFactory{
		options:  options,
		fs:       fs,
		platform: platform,
		logger:   logger,
	}
}

func (f SettingsSourceFactory) New() (SettingsSource, error) {
	if f.options.UseRegistry {
		return f.buildWithRegistry()
	}

	return f.buildWithoutRegistry()
}

func (f SettingsSourceFactory) buildWithRegistry() (SettingsSource, error) {
	var metadataServices []MetadataService

	digDNSResolver := NewDigDNSResolver(f.platform.GetRunner(), f.logger)
	resolver := NewRegistryEndpointResolver(digDNSResolver)

	for _, opts := range f.options.Sources {
		var metadataService MetadataService

		switch typedOpts := opts.(type) {
		case HTTPSourceOptions:
			metadataService = NewHTTPMetadataService(typedOpts.URI, resolver)

		case ConfigDriveSourceOptions:
			metadataService = NewConfigDriveMetadataService(
				resolver,
				f.platform,
				typedOpts.DiskPaths,
				typedOpts.MetaDataPath,
				typedOpts.UserDataPath,
				f.logger,
			)

		case FileSourceOptions:
			metadataService = NewFileMetadataService(
				typedOpts.MetaDataPath,
				typedOpts.UserDataPath,
				typedOpts.SettingsPath,
				f.fs,
				f.logger,
			)

		case CDROMSourceOptions:
			return nil, bosherr.Error("CDROM source is not supported when registry is used")
		}

		metadataServices = append(metadataServices, metadataService)
	}

	metadataService := NewMultiSourceMetadataService(metadataServices...)
	registryProvider := NewRegistryProvider(metadataService, f.options.UseServerName, f.fs, f.logger)
	settingsSource := NewComplexSettingsSource(metadataService, registryProvider, f.logger)

	return settingsSource, nil
}

func (f SettingsSourceFactory) buildWithoutRegistry() (SettingsSource, error) {
	var settingsSources []SettingsSource

	for _, opts := range f.options.Sources {
		var settingsSource SettingsSource

		switch typedOpts := opts.(type) {
		case HTTPSourceOptions:
			return nil, bosherr.Error("HTTP source is not supported without registry")

		case ConfigDriveSourceOptions:
			settingsSource = NewConfigDriveSettingsSource(
				typedOpts.DiskPaths,
				typedOpts.MetaDataPath,
				typedOpts.SettingsPath,
				f.platform,
				f.logger,
			)

		case FileSourceOptions:
			return nil, bosherr.Error("File source is not supported without registry")

		case CDROMSourceOptions:
			settingsSource = NewCDROMSettingsSource(
				typedOpts.FileName,
				f.platform,
				f.logger,
			)
		}

		settingsSources = append(settingsSources, settingsSource)
	}

	return NewMultiSettingsSource(settingsSources...)
}

func (s *SourceOptionsSlice) UnmarshalJSON(data []byte) error {
	var maps []map[string]interface{}

	err := json.Unmarshal(data, &maps)
	if err != nil {
		return bosherr.WrapError(err, "Unmarshalling sources")
	}

	for _, m := range maps {
		if optType, ok := m["Type"]; ok {
			var err error
			var opts SourceOptions

			switch {
			case optType == "HTTP":
				var o HTTPSourceOptions
				err, opts = mapstruc.Decode(m, &o), o

			case optType == "ConfigDrive":
				var o ConfigDriveSourceOptions
				err, opts = mapstruc.Decode(m, &o), o

			case optType == "File":
				var o FileSourceOptions
				err, opts = mapstruc.Decode(m, &o), o

			case optType == "CDROM":
				var o CDROMSourceOptions
				err, opts = mapstruc.Decode(m, &o), o

			default:
				err = bosherr.Errorf("Unknown source type '%s'", optType)
			}

			if err != nil {
				return bosherr.WrapErrorf(err, "Unmarshalling source type '%s'", optType)
			}
			*s = append(*s, opts)
		} else {
			return bosherr.Error("Missing source type")
		}
	}

	return nil
}
