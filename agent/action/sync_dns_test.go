package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	fakeblobstore "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuidgen "github.com/cloudfoundry/bosh-utils/uuid/fakes"
)

var _ = Describe("SyncDNS", func() {
	var (
		syncDNS           SyncDNS
		fakeBlobstore     *fakeblobstore.FakeBlobstore
		fakeUUIDGenerator *fakeuuidgen.FakeGenerator
		fakePlatform      *fakeplatform.FakePlatform
		fakeFileSystem    *fakesys.FakeFileSystem
		logger            boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)

		fakeBlobstore = fakeblobstore.NewFakeBlobstore()
		fakePlatform = fakeplatform.NewFakePlatform()
		fakeFileSystem = fakePlatform.GetFs().(*fakesys.FakeFileSystem)
		fakeUUIDGenerator = fakeuuidgen.NewFakeGenerator()

		syncDNS = NewSyncDNS(fakeBlobstore, fakePlatform, fakeUUIDGenerator, logger)
	})

	It("returns IsAsynchronous false", func() {
		async := syncDNS.IsAsynchronous()
		Expect(async).To(BeFalse())
	})

	It("returns IsPersistent false", func() {
		persistent := syncDNS.IsPersistent()
		Expect(persistent).To(BeFalse())
	})

	It("returns error 'Not supported' when resumed", func() {
		result, err := syncDNS.Resume()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Not supported"))
		Expect(result).To(BeNil())
	})

	It("returns error 'Not supported' when canceled", func() {
		err := syncDNS.Cancel()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("Not supported"))
	})

	Context("when sync_dns is recieved", func() {
		Context("when blobstore contains DNS records", func() {
			BeforeEach(func() {
				fakeDNSRecordsString := `
				{
					"records": [
						["fake-ip0", "fake-name0"],
						["fake-ip1", "fake-name1"]
					]
				}`

				err := fakeFileSystem.WriteFileString("fake-blobstore-file-path", fakeDNSRecordsString)
				Expect(err).ToNot(HaveOccurred())

				fakeBlobstore.GetFileName = "fake-blobstore-file-path"
			})

			It("accesses the blobstore and fetches DNS records", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBlobstore.GetBlobIDs).To(ContainElement("fake-blobstore-id"))
				Expect(fakeBlobstore.GetFingerprints).To(ContainElement("fake-fingerprint"))

				Expect(fakeBlobstore.GetError).ToNot(HaveOccurred())
				Expect(fakeBlobstore.GetFileName).ToNot(Equal(""))
			})

			It("reads the DNS records from the blobstore file", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBlobstore.GetError).ToNot(HaveOccurred())
				Expect(fakeBlobstore.GetFileName).To(Equal("fake-blobstore-file-path"))
				Expect(fakeFileSystem.ReadFileError).ToNot(HaveOccurred())
			})

			It("fails reading the DNS records from the blobstore file", func() {
				fakeFileSystem.RegisterReadFileError("fake-blobstore-file-path", errors.New("fake-error"))

				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Reading fileName"))
			})

			It("saves DNS records to the platform", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakePlatform.SaveDNSRecordsError).To(BeNil())
				Expect(fakePlatform.SaveDNSRecordsDNSRecords).To(Equal(boshsettings.DNSRecords{
					Records: [][2]string{
						{"fake-ip0", "fake-name0"},
						{"fake-ip1", "fake-name1"},
					},
				}))
			})

			Context("when DNS records is invalid", func() {
				BeforeEach(func() {
					err := fakeFileSystem.WriteFileString("fake-blobstore-file-path", "")
					Expect(err).ToNot(HaveOccurred())
				})

				It("fails unmarshalling the DNS records from the file", func() {
					_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Unmarshalling DNS records"))
				})
			})

			Context("when platform fails to save DNS records", func() {
				BeforeEach(func() {
					fakePlatform.SaveDNSRecordsError = errors.New("fake-error")
				})

				It("fails to save DNS records on the platform", func() {
					_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Saving DNS records in platform"))
				})
			})
		})

		Context("when blobstore does not contain DNS records", func() {
			It("fails getting the DNS records", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
