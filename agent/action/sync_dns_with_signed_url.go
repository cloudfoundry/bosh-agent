package action

import (
	"encoding/json"
	"errors"
	"path/filepath"
	"sync"

	"github.com/cloudfoundry/bosh-agent/agent/action/state"
	httpblobprovider "github.com/cloudfoundry/bosh-agent/agent/http_blob_provider"
	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuid "github.com/cloudfoundry/bosh-utils/uuid"
)

type SyncDNSWithSignedURLRequest struct {
	SignedURL   string                    `json:"signed_url"`
	MultiDigest boshcrypto.MultipleDigest `json:"multi_digest"`
	Version     uint64                    `json:"version"`
}

type SyncDNSWithSignedURL struct {
	httpBlobProvider httpblobprovider.HTTPBlobProvider
	settingsService  boshsettings.Service
	platform         boshplat.Platform
	logger           boshlog.Logger
	logTag           string
	lock             *sync.Mutex
}

func NewSyncDNSWithSignedURL(
	settingsService boshsettings.Service,
	platform boshplat.Platform,
	logger boshlog.Logger,
	httpBlobProvider httpblobprovider.HTTPBlobProvider,
) (action SyncDNSWithSignedURL) {
	action.settingsService = settingsService
	action.platform = platform
	action.logger = logger
	action.lock = &sync.Mutex{}
	action.logTag = "SyncDNSWithSignedURL"
	action.httpBlobProvider = httpBlobProvider
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

	contents, err := a.httpBlobProvider.Get(request.SignedURL, request.MultiDigest)
	if err != nil {
		return "", bosherr.WrapError(err, "fetching new DNS records")
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
