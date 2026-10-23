package action_test

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/agent/action"
	blobfakes "github.com/cloudfoundry/bosh-agent/v2/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
	settingsfakes "github.com/cloudfoundry/bosh-agent/v2/settings/fakes"
	"github.com/cloudfoundry/bosh-utils/crypto"
	"github.com/cloudfoundry/bosh-utils/logger/loggerfakes"
	fsfakes "github.com/cloudfoundry/bosh-utils/system/fakes"
)

func TestDNSActionsCommitVersionsInOrder(t *testing.T) {
	fs := fsfakes.NewFakeFileSystem()
	if err := fs.WriteFileString("v1", `{"version":1}`); err != nil {
		t.Fatal(err)
	}
	if err := fs.WriteFileString("v2", `{"version":2}`); err != nil {
		t.Fatal(err)
	}
	platform := &platformfakes.FakePlatform{}
	platform.GetFsReturns(fs)
	settingsService := &settingsfakes.FakeSettingsService{}
	logger := &loggerfakes.FakeLogger{}
	lowBlob := &blobfakes.FakeBlobstoreDelegator{}
	lowBlob.GetReturns("v1", nil)
	highBlob := &blobfakes.FakeBlobstoreDelegator{}
	highBlob.GetReturns("v2", nil)
	low := action.NewSyncDNS(lowBlob, settingsService, platform, logger)
	high := action.NewSyncDNSWithSignedURL(settingsService, platform, logger, highBlob)
	lowStarted := make(chan struct{})
	releaseLow := make(chan struct{})
	highSaved := make(chan struct{})
	platform.SaveDNSRecordsStub = func(records settings.DNSRecords, _ string) error {
		if records.Version == 1 {
			close(lowStarted)
			<-releaseLow
		} else {
			close(highSaved)
		}
		return nil
	}
	lowDone := make(chan error, 1)
	highDone := make(chan error, 1)
	go func() { _, err := low.Run("blob", crypto.MultipleDigest{}, 1); lowDone <- err }()
	<-lowStarted
	go func() { _, err := high.Run(action.SyncDNSWithSignedURLRequest{Version: 2}); highDone <- err }()
	outOfOrder := false
	select {
	case <-highSaved:
		outOfOrder = true
	case <-time.After(30 * time.Millisecond):
	}
	close(releaseLow)
	if err := <-lowDone; err != nil {
		t.Fatal(err)
	}
	if err := <-highDone; err != nil {
		t.Fatal(err)
	}
	if outOfOrder {
		t.Fatal("signed DNS update overtook incomplete regular update")
	}
	data, err := fs.ReadFile(filepath.Join(platform.GetDirProvider().InstanceDNSDir(), "records.json"))
	if err != nil || string(data) != `{"version":2}` {
		t.Fatalf("final DNS state: %s %v", data, err)
	}
}
