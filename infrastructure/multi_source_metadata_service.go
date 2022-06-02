package infrastructure

import (
	"errors"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
)

type MultiSourceMetadataService struct {
	Services        []MetadataService
	selectedService MetadataService
}

func NewMultiSourceMetadataService(services ...MetadataService) MetadataService {
	return &MultiSourceMetadataService{Services: services}
}

func (ms *MultiSourceMetadataService) GetPublicKey() (string, error) {
	selectedService, err := ms.getSelectedService()

	if err != nil {
		return "", err
	}

	return selectedService.GetPublicKey()
}

func (ms *MultiSourceMetadataService) GetInstanceID() (string, error) {
	selectedService, err := ms.getSelectedService()

	if err != nil {
		return "", err
	}

	return selectedService.GetInstanceID()
}

func (ms *MultiSourceMetadataService) GetServerName() (string, error) {
	selectedService, err := ms.getSelectedService()

	if err != nil {
		return "", err
	}

	return selectedService.GetServerName()
}

func (ms *MultiSourceMetadataService) GetNetworks() (boshsettings.Networks, error) {
	selectedService, err := ms.getSelectedService()

	if err != nil {
		return boshsettings.Networks{}, err
	}

	return selectedService.GetNetworks()
}

func (ms *MultiSourceMetadataService) GetSettings() (boshsettings.Settings, error) {
	selectedService, err := ms.getSelectedService()

	if err != nil {
		return boshsettings.Settings{}, err
	}

	return selectedService.GetSettings()
}

func (ms *MultiSourceMetadataService) IsAvailable() bool {
	for _, service := range ms.Services {
		if service.IsAvailable() {
			return true
		}
	}

	return false
}

func (ms *MultiSourceMetadataService) getSelectedService() (MetadataService, error) {
	if ms.selectedService == nil {
		for _, service := range ms.Services {
			if service.IsAvailable() {
				ms.selectedService = service
				break
			}
		}
	}

	if ms.selectedService == nil {
		return nil, errors.New("services not available")
	}

	return ms.selectedService, nil
}
