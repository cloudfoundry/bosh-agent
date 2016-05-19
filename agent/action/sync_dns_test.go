package action_test

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	fakeblobstore "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuidgen "github.com/cloudfoundry/bosh-utils/uuid/fakes"
)

var _ = Describe("SyncDNS", func() {
	var (
		syncDNS           SyncDNS
		fakeBlobstore     *fakeblobstore.FakeBlobstore
		fakeUUIDGenerator *fakeuuidgen.FakeGenerator
		fakeFileSystem    *fakesys.FakeFileSystem
		logger            boshlog.Logger
	)

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)

		fakeBlobstore = fakeblobstore.NewFakeBlobstore()
		fakeFileSystem = fakesys.NewFakeFileSystem()
		fakeUUIDGenerator = fakeuuidgen.NewFakeGenerator()

		syncDNS = NewSyncDNS(fakeBlobstore, fakeFileSystem, fakeUUIDGenerator, logger)
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
			const defaultEtcHostsEntries string = `127.0.0.1 localhost

# The following lines are desirable for IPv6 capable hosts
::1 localhost ip6-localhost ip6-loopback
fe00::0 ip6-localnet
ff00::0 ip6-mcastprefix
ff02::1 ip6-allnodes
ff02::2 ip6-allrouters
ff02::3 ip6-allhosts`

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

			It("reads the DNS records from the file", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeBlobstore.GetError).ToNot(HaveOccurred())
				Expect(fakeBlobstore.GetFileName).To(Equal("fake-blobstore-file-path"))
				Expect(fakeFileSystem.ReadFileError).ToNot(HaveOccurred())
			})

			It("preserves the default DNS records in '/etc/hosts'", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				hostsFileContents, err := fakeFileSystem.ReadFile("/etc/hosts")
				Expect(err).ToNot(HaveOccurred())
				Expect(string(hostsFileContents)).To(ContainSubstring(defaultEtcHostsEntries))
			})

			It("writes the new DNS records in '/etc/hosts'", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				hostsFileContents, err := fakeFileSystem.ReadFile("/etc/hosts")
				Expect(err).ToNot(HaveOccurred())

				Expect(hostsFileContents).Should(MatchRegexp("fake-ip0\\s+fake-name0"))
				Expect(hostsFileContents).Should(MatchRegexp("fake-ip1\\s+fake-name1"))
			})

			It("fails generating a UUID", func() {
				fakeUUIDGenerator.GenerateError = errors.New("fake-error")

				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Generating UUID"))
			})

			It("creates intermediary /etc/hosts-uuid file with content and move it atomically to /etc/hosts", func() {
				_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeFileSystem.RenameError).ToNot(HaveOccurred())

				Expect(len(fakeFileSystem.RenameOldPaths)).To(Equal(1))
				Expect(fakeFileSystem.RenameOldPaths).To(ContainElement("/etc/hosts-fake-uuid-0"))

				Expect(len(fakeFileSystem.RenameNewPaths)).To(Equal(1))
				Expect(fakeFileSystem.RenameNewPaths).To(ContainElement("/etc/hosts"))
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

			Context("when failing to write to /etc/hosts", func() {
				BeforeEach(func() {
					fakeFileSystem.WriteFileError = errors.New("fake-error")
				})

				It("writes the new DNS records in '/etc/hosts'", func() {
					_, err := syncDNS.Run("fake-blobstore-id", "fake-fingerprint")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("Writing to /etc/hosts"))
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
