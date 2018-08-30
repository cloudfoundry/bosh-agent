package settings

import (
	"encoding/json"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
	"sync"
)

type Service interface {
	LoadSettings() error

	// GetSettings does not return error because without settings Agent cannot start.
	GetSettings() Settings

	GetPersistentDiskHints() (map[string]DiskSettings, error)

	SavePersistentDiskHint(DiskSettings) error

	RemovePersistentDiskHint(string) error

	PublicSSHKeyForUsername(string) (string, error)

	InvalidateSettings() error
}

const settingsServiceLogTag = "settingsService"

type settingsService struct {
	fs                      boshsys.FileSystem
	settingsPath            string
	settings                Settings
	settingsMutex           sync.Mutex
	persistentDiskHintsPath string
	persistentDiskHintMutex sync.Mutex
	settingsSource          Source
	defaultNetworkResolver  DefaultNetworkResolver
	logger                  boshlog.Logger
}

type DefaultNetworkResolver interface {
	// Ideally we would find a network based on a MAC address
	// but current CPI implementations do not include it
	GetDefaultNetwork() (Network, error)
}

func NewService(
	fs boshsys.FileSystem,
	settingsPath string,
	persistentDiskHintPath string,
	settingsSource Source,
	defaultNetworkResolver DefaultNetworkResolver,
	logger boshlog.Logger,
) Service {
	return &settingsService{
		fs:                      fs,
		settingsPath:            settingsPath,
		settings:                Settings{},
		persistentDiskHintsPath: persistentDiskHintPath,
		settingsSource:          settingsSource,
		defaultNetworkResolver:  defaultNetworkResolver,
		logger:                  logger,
	}
}

func (s *settingsService) PublicSSHKeyForUsername(username string) (string, error) {
	return s.settingsSource.PublicSSHKeyForUsername(username)
}

func (s *settingsService) LoadSettings() error {
	s.logger.Debug(settingsServiceLogTag, "Loading settings from fetcher")

	newSettings, fetchErr := s.settingsSource.Settings()
	if fetchErr != nil {
		s.logger.Error(settingsServiceLogTag, "Failed loading settings via fetcher: %v", fetchErr)

		opts := boshsys.ReadOpts{Quiet: true}
		existingSettingsJSON, readError := s.fs.ReadFileWithOpts(s.settingsPath, opts)
		if readError != nil {
			s.logger.Error(settingsServiceLogTag, "Failed reading settings from file %s", readError.Error())
			return bosherr.WrapError(fetchErr, "Invoking settings fetcher")
		}

		s.logger.Debug(settingsServiceLogTag, "Successfully read settings from file")

		cachedSettings := Settings{}

		err := json.Unmarshal(existingSettingsJSON, &cachedSettings)
		if err != nil {
			s.logger.Error(settingsServiceLogTag, "Failed unmarshalling settings from file %s", err.Error())
			return bosherr.WrapError(fetchErr, "Invoking settings fetcher")
		}

		s.settingsMutex.Lock()
		s.settings = cachedSettings
		s.settingsMutex.Unlock()

		return nil
	}

	s.logger.Debug(settingsServiceLogTag, "Successfully received settings from fetcher")
	s.settingsMutex.Lock()
	s.settings = newSettings
	s.settingsMutex.Unlock()

	newSettingsJSON, err := json.Marshal(newSettings)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling settings json")
	}

	err = s.fs.WriteFileQuietly(s.settingsPath, newSettingsJSON)
	if err != nil {
		return bosherr.WrapError(err, "Writing setting json")
	}

	return nil
}

func (s *settingsService) GetPersistentDiskHints() (map[string]DiskSettings, error) {
	s.persistentDiskHintMutex.Lock()
	defer s.persistentDiskHintMutex.Unlock()
	return s.getPersistentDiskHintsWithoutLocking()
}

func (s *settingsService) RemovePersistentDiskHint(diskID string) error {
	s.persistentDiskHintMutex.Lock()
	defer s.persistentDiskHintMutex.Unlock()

	persistentDiskHints, err := s.getPersistentDiskHintsWithoutLocking()
	if err != nil {
		return bosherr.WrapError(err, "Cannot remove entry from file due to read error")
	}

	delete(persistentDiskHints, diskID)
	if err := s.savePersistentDiskHintsWithoutLocking(persistentDiskHints); err != nil {
		return bosherr.WrapError(err, "Saving persistent disk hints")
	}

	return nil
}

func (s *settingsService) SavePersistentDiskHint(persistentDiskSettings DiskSettings) error {
	s.persistentDiskHintMutex.Lock()
	defer s.persistentDiskHintMutex.Unlock()

	persistentDiskHints, err := s.getPersistentDiskHintsWithoutLocking()
	if err != nil {
		return bosherr.WrapError(err, "Reading all persistent disk hints")
	}

	persistentDiskHints[persistentDiskSettings.ID] = persistentDiskSettings
	if err := s.savePersistentDiskHintsWithoutLocking(persistentDiskHints); err != nil {
		return bosherr.WrapError(err, "Saving persistent disk hints")
	}

	return nil
}

// GetSettings returns setting even if it fails to resolve IPs for dynamic networks.
func (s *settingsService) GetSettings() Settings {
	s.settingsMutex.Lock()

	settingsCopy := s.settings

	if s.settings.Networks != nil {
		settingsCopy.Networks = make(map[string]Network)
	}

	for networkName, network := range s.settings.Networks {
		settingsCopy.Networks[networkName] = network
	}
	s.settingsMutex.Unlock()

	for networkName, network := range settingsCopy.Networks {
		if !network.IsDHCP() {
			continue
		}

		resolvedNetwork, err := s.resolveNetwork(network)
		if err != nil {
			break
		}

		settingsCopy.Networks[networkName] = resolvedNetwork
	}
	return settingsCopy
}

func (s *settingsService) InvalidateSettings() error {
	err := s.fs.RemoveAll(s.settingsPath)
	if err != nil {
		return bosherr.WrapError(err, "Removing settings file")
	}

	return nil
}

func (s *settingsService) resolveNetwork(network Network) (Network, error) {
	// Ideally this would be GetNetworkByMACAddress(mac string)
	// Currently, we are relying that if the default network does not contain
	// the MAC adddress the InterfaceConfigurationCreator will fail.
	resolvedNetwork, err := s.defaultNetworkResolver.GetDefaultNetwork()
	if err != nil {
		s.logger.Error(settingsServiceLogTag, "Failed retrieving default network %s", err.Error())
		return Network{}, bosherr.WrapError(err, "Failed retrieving default network")
	}

	// resolvedNetwork does not have all information for a network
	network.IP = resolvedNetwork.IP
	network.Netmask = resolvedNetwork.Netmask
	network.Gateway = resolvedNetwork.Gateway
	network.Resolved = true

	return network, nil
}

func (s *settingsService) savePersistentDiskHintsWithoutLocking(persistentDiskHints map[string]DiskSettings) error {
	newPersistentDiskHintsJSON, err := json.Marshal(persistentDiskHints)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling persistent disk hints json")
	}

	err = s.fs.WriteFile(s.persistentDiskHintsPath, newPersistentDiskHintsJSON)
	if err != nil {
		return bosherr.WrapError(err, "Writing persistent disk hints settings json")
	}

	return nil
}

func (s *settingsService) getPersistentDiskHintsWithoutLocking() (map[string]DiskSettings, error) {
	persistentDiskHints := make(map[string]DiskSettings)

	if s.fs.FileExists(s.persistentDiskHintsPath) {
		opts := boshsys.ReadOpts{Quiet: true}
		existingSettingsJSON, readError := s.fs.ReadFileWithOpts(s.persistentDiskHintsPath, opts)
		if readError != nil {
			return nil, bosherr.WrapError(readError, "Reading persistent disk hints from file")
		}

		err := json.Unmarshal(existingSettingsJSON, &persistentDiskHints)
		if err != nil {
			return nil, bosherr.WrapError(err, "Unmarshalling persistent disk hints from file")
		}
	}
	return persistentDiskHints, nil
}
