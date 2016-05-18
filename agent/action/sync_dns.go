package action

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"

	boshblob "github.com/cloudfoundry/bosh-utils/blobstore"
	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"
)

const defaultEtcHostsEntries string = `127.0.0.1 localhost

# The following lines are desirable for IPv6 capable hosts
::1 localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts`

type SyncDNS struct {
	blobstore boshblob.Blobstore
	fs        boshsys.FileSystem
	logger    boshlog.Logger
}

type DNSRecords struct {
	Records [][2]string `json:"records"`
}

func NewSyncDNS(blobstore boshblob.Blobstore, fs boshsys.FileSystem, logger boshlog.Logger) SyncDNS {
	return SyncDNS{
		blobstore: blobstore,
		fs:        fs,
		logger:    logger,
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

	contents, err := a.fs.ReadFile(fileName)
	if err != nil {
		return nil, bosherr.WrapError(err, fmt.Sprintf("Reading fileName %s from blobstore", fileName))
	}

	dnsRecords := DNSRecords{}
	err = json.Unmarshal(contents, &dnsRecords)
	if err != nil {
		return nil, bosherr.WrapError(err, fmt.Sprintf("Unmarshalling DNS records"))
	}

	dnsRecordsContents := bytes.Buffer{}
	dnsRecordsContents.WriteString(defaultEtcHostsEntries + "\n")

	for _, dnsRecord := range dnsRecords.Records {
		dnsRecordsContents.WriteString(fmt.Sprintf("%s %s", dnsRecord[0], dnsRecord[1]))
	}

	err = a.fs.WriteFile("/etc/hosts", dnsRecordsContents.Bytes())
	if err != nil {
		return nil, bosherr.WrapError(err, fmt.Sprintf("Writing to /etc/hosts"))
	}

	return nil, nil
}
