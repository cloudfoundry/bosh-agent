package settings

import (
	"encoding/json"
	"sync"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

type Service interface {
	LoadSettings() error

	// GetSettings does not return error because without settings Agent cannot start.
	GetSettings() Settings

	GetPersistentDiskSettings(diskCID string) (DiskSettings, error)

	GetAllPersistentDiskSettings() (map[string]DiskSettings, error)

	SavePersistentDiskSettings(DiskSettings) error

	RemovePersistentDiskSettings(string) error

	PublicSSHKeyForUsername(string) (string, error)

	InvalidateSettings() error

	SaveUpdateSettings(updateSettings UpdateSettings) error
}

const settingsServiceLogTag = "settingsService"

type settingsService struct {
	fs                          boshsys.FileSystem
	settings                    Settings
	settingsMutex               sync.Mutex
	persistentDiskSettingsMutex sync.Mutex
	settingsSource              Source
	platform                    PlatformSettingsGetter
	logger                      boshlog.Logger
}

type DefaultNetworkResolver interface {
	// Ideally we would find a network based on a MAC address
	// but current CPI implementations do not include it
	GetDefaultNetwork() (Network, error)
}

//go:generate go run github.com/maxbrunsfeld/counterfeiter/v6 . PlatformSettingsGetter

type PlatformSettingsGetter interface {
	DefaultNetworkResolver
	SetupBoshSettingsDisk() error
	GetAgentSettingsPath(tmpfs bool) string
	GetPersistentDiskSettingsPath(tmpfs bool) string
	GetUpdateSettingsPath(tmpfs bool) string
}

func NewService(
	fs boshsys.FileSystem,
	settingsSource Source,
	platform PlatformSettingsGetter,
	logger boshlog.Logger,
) Service {
	return &settingsService{
		fs:             fs,
		settings:       Settings{},
		settingsSource: settingsSource,
		platform:       platform,
		logger:         logger,
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
		existingSettingsJSON, readError := s.fs.ReadFileWithOpts(s.getSettingsPath(), opts)
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

		newUpdateSettings, err := s.getUpdateSettings(cachedSettings.Env.Bosh.Agent.Settings.TmpFS)
		if err != nil {
			s.logger.Error(settingsServiceLogTag, err.Error())
			return err
		}
		cachedSettings.UpdateSettings = newUpdateSettings

		s.settingsMutex.Lock()
		s.settings = cachedSettings
		s.settingsMutex.Unlock()

		return nil
	}

	s.logger.Debug(settingsServiceLogTag, "Successfully received settings from fetcher")
	newUpdateSettings, err := s.getUpdateSettings(newSettings.Env.Bosh.Agent.Settings.TmpFS)
	if err != nil {
		s.logger.Error(settingsServiceLogTag, err.Error())
		return err
	}
	newSettings.UpdateSettings = newUpdateSettings

	s.settingsMutex.Lock()
	s.settings = newSettings
	s.settingsMutex.Unlock()

	if s.settings.Env.Bosh.Agent.Settings.TmpFS {
		if err := s.platform.SetupBoshSettingsDisk(); err != nil {
			return bosherr.WrapError(err, "Setting up settings tmpfs")
		}
	}

	newSettingsJSON, err := json.Marshal(s.settings)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling settings json")
	}

	err = s.fs.WriteFileQuietly(s.getSettingsPath(), newSettingsJSON)
	if err != nil {
		return bosherr.WrapError(err, "Writing setting json")
	}

	return nil
}

func (s *settingsService) GetAllPersistentDiskSettings() (map[string]DiskSettings, error) {
	s.persistentDiskSettingsMutex.Lock()
	defer s.persistentDiskSettingsMutex.Unlock()

	allPersistentDiskSettings := map[string]DiskSettings{}

	settings := s.GetSettings()

	for diskCID, settings := range settings.Disks.Persistent {
		allPersistentDiskSettings[diskCID] = s.settings.populatePersistentDiskSettings(diskCID, settings)
	}

	persistentDiskSettings, err := s.getPersistentDiskSettingsWithoutLocking()
	if err != nil {
		return nil, bosherr.WrapError(err, "Reading persistent disk settings")
	}

	for diskCID, settings := range persistentDiskSettings {
		allPersistentDiskSettings[diskCID] = settings
	}

	return allPersistentDiskSettings, nil
}

func (s *settingsService) GetPersistentDiskSettings(diskCID string) (DiskSettings, error) {
	allDiskSettings, err := s.GetAllPersistentDiskSettings()
	if err != nil {
		return DiskSettings{}, bosherr.WrapError(err, "Getting all persistent disk settings")
	}

	settings, hasDiskSettings := allDiskSettings[diskCID]
	if !hasDiskSettings {
		return DiskSettings{}, bosherr.Errorf("Persistent disk with volume id '%s' could not be found", diskCID)
	}

	return settings, nil
}

func (s *settingsService) RemovePersistentDiskSettings(diskID string) error {
	s.persistentDiskSettingsMutex.Lock()
	defer s.persistentDiskSettingsMutex.Unlock()

	persistentDiskSettings, err := s.getPersistentDiskSettingsWithoutLocking()
	if err != nil {
		return bosherr.WrapError(err, "Cannot remove entry from file due to read error")
	}

	delete(persistentDiskSettings, diskID)
	if err := s.savePersistentDiskSettingsWithoutLocking(persistentDiskSettings); err != nil {
		return bosherr.WrapError(err, "Saving persistent disk settings")
	}

	return nil
}

func (s *settingsService) SavePersistentDiskSettings(newDiskSettings DiskSettings) error {
	s.persistentDiskSettingsMutex.Lock()
	defer s.persistentDiskSettingsMutex.Unlock()

	persistentDiskSettings, err := s.getPersistentDiskSettingsWithoutLocking()
	if err != nil {
		return bosherr.WrapError(err, "Reading all persistent disk settings")
	}

	persistentDiskSettings[newDiskSettings.ID] = newDiskSettings
	if err := s.savePersistentDiskSettingsWithoutLocking(persistentDiskSettings); err != nil {
		return bosherr.WrapError(err, "Saving persistent disk settings")
	}

	return nil
}

func (s *settingsService) SaveUpdateSettings(updateSettings UpdateSettings) error {
	currentSettings := s.GetSettings()
	updateSettingsPath := s.platform.GetUpdateSettingsPath(currentSettings.Env.Bosh.Agent.Settings.TmpFS)

	updateSettingsJSON, err := json.Marshal(updateSettings)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling Update Settings json")
	}

	err = s.fs.WriteFile(updateSettingsPath, updateSettingsJSON)
	if err != nil {
		return bosherr.WrapError(err, "Writing Update Settings json")
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
	err := s.fs.RemoveAll(s.getSettingsPath())
	if err != nil {
		return bosherr.WrapError(err, "Removing settings file")
	}

	return nil
}

func (s *settingsService) resolveNetwork(network Network) (Network, error) {
	// Ideally this would be GetNetworkByMACAddress(mac string)
	// Currently, we are relying that if the default network does not contain
	// the MAC adddress the InterfaceConfigurationCreator will fail.
	resolvedNetwork, err := s.platform.GetDefaultNetwork()
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

func (s *settingsService) savePersistentDiskSettingsWithoutLocking(persistentDiskSettings map[string]DiskSettings) error {
	newPersistentDiskSettingsJSON, err := json.Marshal(persistentDiskSettings)
	if err != nil {
		return bosherr.WrapError(err, "Marshalling persistent disk settings json")
	}

	err = s.fs.WriteFile(s.getPersistentDiskSettingsPath(), newPersistentDiskSettingsJSON)
	if err != nil {
		return bosherr.WrapError(err, "Writing persistent disk settings settings json")
	}

	return nil
}

func (s *settingsService) getPersistentDiskSettingsWithoutLocking() (map[string]DiskSettings, error) {
	persistentDiskSettings := make(map[string]DiskSettings)

	if s.fs.FileExists(s.getPersistentDiskSettingsPath()) {
		opts := boshsys.ReadOpts{Quiet: true}
		existingSettingsJSON, readError := s.fs.ReadFileWithOpts(s.getPersistentDiskSettingsPath(), opts)
		if readError != nil {
			return nil, bosherr.WrapError(readError, "Reading persistent disk settings from file")
		}

		err := json.Unmarshal(existingSettingsJSON, &persistentDiskSettings)
		if err != nil {
			return nil, bosherr.WrapError(err, "Unmarshalling persistent disk settings from file")
		}
	}
	return persistentDiskSettings, nil
}

func (s *settingsService) getUpdateSettings(useTmpFS bool) (UpdateSettings, error) {
	updateSettings := UpdateSettings{}
	updateSettingsPath := s.platform.GetUpdateSettingsPath(useTmpFS)
	if s.fs.FileExists(updateSettingsPath) {
		existingSettingsContents, err := s.fs.ReadFile(updateSettingsPath)
		if err != nil {
			return updateSettings, bosherr.WrapError(err, "Reading Update Settings json")
		}
		err = json.Unmarshal(existingSettingsContents, &updateSettings)
		if err != nil {
			return updateSettings, bosherr.WrapError(err, "Unmarshalling Update Settings json")
		}
	}
	return updateSettings, nil
}

func (s *settingsService) getSettingsPath() string {
	s.settingsMutex.Lock()
	defer s.settingsMutex.Unlock()

	return s.platform.GetAgentSettingsPath(s.settings.Env.Bosh.Agent.Settings.TmpFS)
}

func (s *settingsService) getPersistentDiskSettingsPath() string {
	s.settingsMutex.Lock()
	defer s.settingsMutex.Unlock()

	return s.platform.GetPersistentDiskSettingsPath(s.settings.Env.Bosh.Agent.Settings.TmpFS)
}
