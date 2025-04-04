package action

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"sync"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action/state"
	blobdelegator "github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator"
	boshplat "github.com/cloudfoundry/bosh-agent/v2/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

type SyncDNSWithSignedURLRequest struct {
	SignedURL        string                    `json:"signed_url"`
	MultiDigest      boshcrypto.MultipleDigest `json:"multi_digest"`
	Version          uint64                    `json:"version"`
	BlobstoreHeaders map[string]string         `json:"blobstore_headers"`
}

type SyncDNSWithSignedURL struct {
	blobDelegator   blobdelegator.BlobstoreDelegator
	settingsService boshsettings.Service
	platform        boshplat.Platform
	logger          boshlog.Logger
	logTag          string
	lock            *sync.Mutex
}

func NewSyncDNSWithSignedURL(
	settingsService boshsettings.Service,
	platform boshplat.Platform,
	logger boshlog.Logger,
	bd blobdelegator.BlobstoreDelegator) (action SyncDNSWithSignedURL) {
	action.settingsService = settingsService
	action.platform = platform
	action.logger = logger
	action.lock = &sync.Mutex{}
	action.logTag = "SyncDNSWithSignedURL"
	action.blobDelegator = bd
	return
}

func (a SyncDNSWithSignedURL) IsAsynchronous(_ ProtocolVersion) bool {
	return false
}

func (a SyncDNSWithSignedURL) IsPersistent() bool {
	return false
}

func (a SyncDNSWithSignedURL) IsLoggable() bool {
	return true
}

func (a SyncDNSWithSignedURL) Resume() (interface{}, error) {
	return nil, errors.New("not supported")
}

func (a SyncDNSWithSignedURL) Cancel() error {
	return errors.New("not supported")
}

func (a SyncDNSWithSignedURL) Run(request SyncDNSWithSignedURLRequest) (string, error) {
	if !a.needsUpdateWithLock(request.Version) {
		return "synced", nil
	}

	filePath, err := a.blobDelegator.Get(request.MultiDigest, request.SignedURL, "", request.BlobstoreHeaders)
	if err != nil {
		return "", bosherr.WrapError(err, "fetching new DNS records")
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

	syncDNSState := a.createSyncDNSState()
	if !syncDNSState.NeedsUpdate(request.Version) {
		return "synced", nil
	}

	dnsRecords := boshsettings.DNSRecords{}
	if err := json.Unmarshal(contents, &dnsRecords); err != nil {
		return "", bosherr.WrapError(err, "unmarshalling DNS records")
	}

	if dnsRecords.Version != request.Version {
		return "", bosherr.Error("version from unpacked dns blob does not match version supplied by director")
	}

	err = a.platform.SaveDNSRecords(dnsRecords, a.settingsService.GetSettings().AgentID)
	if err != nil {
		return "", bosherr.WrapError(err, "saving DNS records")
	}

	err = syncDNSState.SaveState(contents)
	if err != nil {
		return "", bosherr.WrapError(err, "saving local DNS state")
	}

	return "synced", nil
}

func (a SyncDNSWithSignedURL) createSyncDNSState() state.SyncDNSState {
	stateFilePath := filepath.Join(a.platform.GetDirProvider().InstanceDNSDir(), localDNSStateFilename)
	return state.NewSyncDNSState(a.platform, stateFilePath, boshuuid.NewGenerator())
}

func (a SyncDNSWithSignedURL) needsUpdateWithLock(version uint64) bool {
	a.lock.Lock()
	defer a.lock.Unlock()

	return a.createSyncDNSState().NeedsUpdate(version)
}
