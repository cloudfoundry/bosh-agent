package action

import (
	"encoding/json"
	"errors"
	"fmt"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"

	boshplat "github.com/cloudfoundry/bosh-agent/platform"
	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshuuidgen "github.com/cloudfoundry/bosh-utils/uuid"
)

type SyncDNS struct {
	blobstore     boshblob.Blobstore
	uuidGenerator boshuuidgen.Generator
	platform      boshplat.Platform
	logger        boshlog.Logger
}

func NewSyncDNS(blobstore boshblob.Blobstore, platform boshplat.Platform, uuidGenerator boshuuidgen.Generator, logger boshlog.Logger) SyncDNS {
	return SyncDNS{
		blobstore:     blobstore,
		uuidGenerator: uuidGenerator,
		platform:      platform,
		logger:        logger,
	}
}

func (a SyncDNS) IsAsynchronous() bool {
	return false
}

func (a SyncDNS) IsPersistent() bool {
	return false
}

func (a SyncDNS) Resume() (interface{}, error) {
	return nil, errors.New("Not supported")
}

func (a SyncDNS) Cancel() error {
	return errors.New("Not supported")
}

func (a SyncDNS) Run(blobID, sha1 string) (interface{}, error) {
	fileName, err := a.blobstore.Get(blobID, sha1)
	if err != nil {
		return nil, bosherr.WrapError(err, fmt.Sprintf("Getting %s from blobstore", blobID))
	}

	contents, err := a.platform.GetFs().ReadFile(fileName)
	if err != nil {
		return nil, bosherr.WrapError(err, fmt.Sprintf("Reading fileName %s from blobstore", fileName))
	}

	dnsRecords := boshsettings.DNSRecords{}
	err = json.Unmarshal(contents, &dnsRecords)
	if err != nil {
		return nil, bosherr.WrapError(err, "Unmarshalling DNS records")
	}

	err = a.platform.SaveDNSRecords(dnsRecords)
	if err != nil {
		return nil, bosherr.WrapError(err, "Saving DNS records in platform")
	}

	return nil, nil
}
