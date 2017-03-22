package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/cloudfoundry/bosh-agent/agent/action/state"

	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

const localDNSStateFilename = "records.json"

type SyncDNS struct {
	blobstore       boshblob.DigestBlobstore
	settingsService boshsettings.Service
	platform        boshplat.Platform
	logger          boshlog.Logger
	logTag          string
	lock            *sync.Mutex
}

func NewSyncDNS(blobstore boshblob.DigestBlobstore, settingsService boshsettings.Service, platform boshplat.Platform, logger boshlog.Logger) SyncDNS {
	return SyncDNS{
		blobstore:       blobstore,
		settingsService: settingsService,
		platform:        platform,
		logger:          logger,
		lock:            &sync.Mutex{},
		logTag:          "Sync DNS action",
	}
}

func (a SyncDNS) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a SyncDNS) IsPersistent() bool {
	return false
}

func (a SyncDNS) IsLoggable() bool {
	return true
}

func (a SyncDNS) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a SyncDNS) Cancel() error {
	return errors.New("not supported")
}

func (a SyncDNS) Run(blobID string, multiDigest boshcrypto.MultipleDigest, version uint64) (string, error) {
	requestVersionStale, err := a.isLocalStateGreaterThanOrEqual(version)
	if err != nil {
		return "", bosherr.WrapError(err, "reading local DNS state")
	}

	if requestVersionStale {
		return "synced", nil
	}

	filePath, err := a.blobstore.Get(blobID, multiDigest)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "getting %s from blobstore", blobID)
	}

	fs := a.platform.GetFs()

	defer func() {
		err = fs.RemoveAll(filePath)
		if err != nil {
			a.logger.Error(a.logTag, fmt.Sprintf("Failed to remove dns blob file at path '%s'", filePath))
		}
	}()

	contents, err := fs.ReadFile(filePath)
	if err != nil {
		return "", bosherr.WrapErrorf(err, "reading %s from blobstore", filePath)
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	localDNSState := state.LocalDNSState{}
	syncDNSState := a.createSyncDNSState()
	if syncDNSState.StateFileExists() {
		localDNSState, err = syncDNSState.LoadState()
		if err != nil {
			return "", bosherr.WrapError(err, "loading local DNS state")
		}
	}

	//Checking again since don't want to keep lock during blobstore operations
	if localDNSState.Version >= version {
		return "synced", nil
	}

	if err := json.Unmarshal(contents, &localDNSState); err != nil {
		return "", bosherr.WrapError(err, "unmarshalling DNS records")
	}

	if localDNSState.Version != version {
		return "", bosherr.Error("version from unpacked dns blob does not match version supplied by director")
	}

	dnsRecords := boshsettings.DNSRecords{
		Version: localDNSState.Version,
		Records: localDNSState.Records,
	}

	err = a.platform.SaveDNSRecords(dnsRecords, a.settingsService.GetSettings().AgentID)
	if err != nil {
		return "", bosherr.WrapError(err, "saving DNS records")
	}

	err = syncDNSState.SaveState(localDNSState)
	if err != nil {
		return "", bosherr.WrapError(err, "saving local DNS state")
	}

	return "synced", nil
}

func (a SyncDNS) createSyncDNSState() state.SyncDNSState {
	stateFilePath := filepath.Join(a.platform.GetDirProvider().InstanceDNSDir(), localDNSStateFilename)
	return state.NewSyncDNSState(a.platform, stateFilePath, boshuuid.NewGenerator())
}

func (a SyncDNS) isLocalStateGreaterThanOrEqual(version uint64) (bool, error) {
	a.lock.Lock()
	defer a.lock.Unlock()

	syncDNSState := a.createSyncDNSState()

	if !syncDNSState.StateFileExists() {
		return false, nil
	}

	localDNSState, err := syncDNSState.LoadState()
	if err != nil {
		return false, bosherr.WrapError(err, "loading local DNS state")
	}

	return (localDNSState.Version >= version), nil
}
