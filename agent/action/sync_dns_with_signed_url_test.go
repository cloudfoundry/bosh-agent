package action_test

import (
	"errors"
	"path/filepath"

	. "github.com/cloudfoundry/bosh-agent/agent/action"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/platformfakes"

	fakeblobdelegator "github.com/cloudfoundry/bosh-agent/agent/httpblobprovider/blobstore_delegator/blobstore_delegatorfakes"
	fakelogger "github.com/cloudfoundry/bosh-agent/logger/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"
	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("SyncDNSWithSignedURL", func() {
	var (
		action               SyncDNSWithSignedURL
		fakeSettingsService  *fakesettings.FakeSettingsService
		fakePlatform         *platformfakes.FakePlatform
		fakeFileSystem       *fakesys.FakeFileSystem
		logger               *fakelogger.FakeLogger
		fakeDNSRecordsString string
		blobDelegator        *fakeblobdelegator.FakeBlobstoreDelegator
	)

	BeforeEach(func() {
		logger = &fakelogger.FakeLogger{}
		blobDelegator = &fakeblobdelegator.FakeBlobstoreDelegator{}
		fakeSettingsService = &fakesettings.FakeSettingsService{}
		fakePlatform = &platformfakes.FakePlatform{}
		fakeFileSystem = fakesys.NewFakeFileSystem()
		fakePlatform.GetFsReturns(fakeFileSystem)

		action = NewSyncDNSWithSignedURL(fakeSettingsService, fakePlatform, logger, blobDelegator)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("#Run", func() {
		var (
			stateFilePath string
			multiDigest   boshcrypto.MultipleDigest
		)

		BeforeEach(func() {
			fakeDNSRecordsString = `
							{
								"version": 2,
								"records": [
									["fake-ip0", "fake-name0"],
									["fake-ip1", "fake-name1"]
								],
								"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
								"record_infos": [
									["id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
								]
							}`
			multiDigest = boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, "fake-fingerprint"))
			err := fakeFileSystem.WriteFileString("fake-blobstore-file-path", fakeDNSRecordsString)
			Expect(err).ToNot(HaveOccurred())
			blobDelegator.GetReturns("fake-blobstore-file-path", nil)

			stateFilePath = filepath.Join(fakePlatform.GetDirProvider().InstanceDNSDir(), "records.json")
		})

		Context("when local DNS state version is >= Run's version", func() {
			BeforeEach(func() {
				blobDelegator.GetReturns("fake-blobstore-file-path", errors.New("fake-blobstore-get-error"))
			})

			Context("when the version equals the Run's version", func() {
				BeforeEach(func() {
					err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 2}`)
					Expect(err).ToNot(HaveOccurred())

					fakeFileSystem.WriteFileError = errors.New("fake-write-error")
				})

				It("returns with no error and does no writes and no gets to blobstore", func() {
					_, err := action.Run(SyncDNSWithSignedURLRequest{
						SignedURL:   "fake-signed-url",
						MultiDigest: multiDigest,
						Version:     2,
					})
					Expect(err).ToNot(HaveOccurred())
				})
			})

			Context("when the version > the Run's version", func() {
				BeforeEach(func() {
					err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 3}`)
					Expect(err).ToNot(HaveOccurred())

					fakeFileSystem.WriteFileError = errors.New("fake-write-error")
				})

				It("returns error", func() {
					_, err := action.Run(SyncDNSWithSignedURLRequest{
						SignedURL:   "fake-signed-url",
						MultiDigest: multiDigest,
						Version:     2,
					})
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when the version in the blob does not match the version director supplied", func() {
			It("returns an error", func() {
				_, err := action.Run(SyncDNSWithSignedURLRequest{
					SignedURL:   "fake-signed-url",
					MultiDigest: multiDigest,
					Version:     3,
				})
				Expect(err).To(MatchError("version from unpacked dns blob does not match version supplied by director"))
			})
		})

		Context("when local DNS state version is < Run's version", func() {
			BeforeEach(func() {
				err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 1}`)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when blobstore contains DNS records", func() {
				It("accesses the blobstore and fetches DNS records", func() {
					headers := map[string]string{
						"key": "value",
					}
					response, err := action.Run(SyncDNSWithSignedURLRequest{
						SignedURL:        "fake-signed-url",
						MultiDigest:      multiDigest,
						Version:          2,
						BlobstoreHeaders: headers,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(blobDelegator.GetCallCount()).To(Equal(1))
					_, signedURL, blobID, urlHeaders := blobDelegator.GetArgsForCall(0)
					Expect(signedURL).To(Equal("fake-signed-url"))
					Expect(blobID).To(BeEmpty())
					Expect(urlHeaders).To(Equal(headers))
				})

				It("reads the DNS records from the blobstore file", func() {
					response, err := action.Run(SyncDNSWithSignedURLRequest{
						SignedURL:   "fake-signed-url",
						MultiDigest: multiDigest,
						Version:     2,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(fakeFileSystem.ReadFileError).ToNot(HaveOccurred())
				})

				It("saves DNS records to the platform", func() {
					response, err := action.Run(SyncDNSWithSignedURLRequest{
						SignedURL:   "fake-signed-url",
						MultiDigest: multiDigest,
						Version:     2,
					})
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(fakePlatform.SaveDNSRecordsCallCount()).To(Equal(1))
					dnsRecords, agentID := fakePlatform.SaveDNSRecordsArgsForCall(0)
					Expect(dnsRecords).To(Equal(boshsettings.DNSRecords{
						Version: 2,
						Records: [][2]string{
							{"fake-ip0", "fake-name0"},
							{"fake-ip1", "fake-name1"},
						},
					}))
					Expect(agentID).To(Equal(""))
				})

				Context("when there is no local DNS state", func() {
					BeforeEach(func() {
						err := fakeFileSystem.RemoveAll(stateFilePath)
						Expect(err).NotTo(HaveOccurred())
					})

					It("saves DNS records to the platform", func() {
						Expect(fakeFileSystem.FileExists(stateFilePath)).To(BeFalse())

						response, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(response).To(Equal("synced"))

						Expect(fakePlatform.SaveDNSRecordsCallCount()).To(Equal(1))
						dnsRecords, agentID := fakePlatform.SaveDNSRecordsArgsForCall(0)
						Expect(dnsRecords).To(Equal(boshsettings.DNSRecords{
							Version: 2,
							Records: [][2]string{
								{"fake-ip0", "fake-name0"},
								{"fake-ip1", "fake-name1"},
							},
						}))
						Expect(agentID).To(Equal(""))
					})
				})

				Context("when there is an error reading the local dns state", func() {
					BeforeEach(func() {
						err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 1}`)
						Expect(err).ToNot(HaveOccurred())

						fakeFileSystem.RegisterReadFileError(stateFilePath, errors.New("fake-read-error"))
					})

					It("saves DNS records to the platform", func() {
						response, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(response).To(Equal("synced"))

						Expect(fakePlatform.SaveDNSRecordsCallCount()).To(Equal(1))
						dnsRecords, agentID := fakePlatform.SaveDNSRecordsArgsForCall(0)
						Expect(dnsRecords).To(Equal(boshsettings.DNSRecords{
							Version: 2,
							Records: [][2]string{
								{"fake-ip0", "fake-name0"},
								{"fake-ip1", "fake-name1"},
							},
						}))
						Expect(agentID).To(Equal(""))
					})
				})

				Context("when the the local dns state is corrupt", func() {
					BeforeEach(func() {
						err := fakeFileSystem.WriteFileString(stateFilePath, "hot-trash")
						Expect(err).ToNot(HaveOccurred())
					})

					It("saves DNS records to the platform", func() {
						response, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).ToNot(HaveOccurred())
						Expect(response).To(Equal("synced"))

						Expect(fakePlatform.SaveDNSRecordsCallCount()).To(Equal(1))
						dnsRecords, agentID := fakePlatform.SaveDNSRecordsArgsForCall(0)
						Expect(dnsRecords).To(Equal(boshsettings.DNSRecords{
							Version: 2,
							Records: [][2]string{
								{"fake-ip0", "fake-name0"},
								{"fake-ip1", "fake-name1"},
							},
						}))
						Expect(agentID).To(Equal(""))
					})
				})

				Context("local DNS state operations", func() {
					Context("when there is no local DNS state", func() {
						BeforeEach(func() {
							err := fakeFileSystem.RemoveAll(stateFilePath)
							Expect(err).NotTo(HaveOccurred())
						})

						It("runs successfully and creates a new state file", func() {
							Expect(fakeFileSystem.FileExists(stateFilePath)).To(BeFalse())

							response, err := action.Run(SyncDNSWithSignedURLRequest{
								SignedURL:   "fake-signed-url",
								MultiDigest: multiDigest,
								Version:     2,
							})
							Expect(err).ToNot(HaveOccurred())
							Expect(response).To(Equal("synced"))

							contents, err := fakeFileSystem.ReadFile(stateFilePath)
							Expect(err).ToNot(HaveOccurred())
							Expect(contents).To(MatchJSON(`
							{
								"version": 2,
								"records": [
									["fake-ip0", "fake-name0"],
									["fake-ip1", "fake-name1"]
								],
								"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
								"record_infos": [
									["id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
								]
							}`))
						})
					})

					Context("when saving fails", func() {
						BeforeEach(func() {
							fakeFileSystem.WriteFileError = errors.New("fake-write-error")
						})

						It("returns an error", func() {
							_, err := action.Run(SyncDNSWithSignedURLRequest{
								SignedURL:   "fake-signed-url",
								MultiDigest: multiDigest,
								Version:     2,
							})
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("saving local DNS state"))
						})
					})
				})

				Context("when DNS records is invalid", func() {
					BeforeEach(func() {
						fakeFileSystem.WriteFileString("fake-blobstore-file-path", "")
					})

					It("fails unmarshalling the DNS records from the file", func() {
						_, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("unmarshalling DNS records"))
					})
				})

				Context("when platform fails to save DNS records", func() {
					BeforeEach(func() {
						fakePlatform.SaveDNSRecordsReturns(errors.New("fake-error"))
						err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 1}`)
						Expect(err).ToNot(HaveOccurred())
					})

					It("fails to save DNS records on the platform", func() {
						_, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("saving DNS records"))
					})

					It("should not update the records.json", func() {
						_, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("saving DNS records"))

						contents, err := fakeFileSystem.ReadFile(stateFilePath)
						Expect(err).ToNot(HaveOccurred())
						Expect(contents).To(MatchJSON(`{"version": 1}`))
					})
				})
			})

			Context("when new DNS records cannot be fetched", func() {
				BeforeEach(func() {
					blobDelegator.GetReturns("fake-blobstore-file-path", errors.New("embedded error"))
				})

				Context("when blobstore returns an error", func() {
					It("fails with an wrapped error", func() {
						_, err := action.Run(SyncDNSWithSignedURLRequest{
							SignedURL:   "fake-signed-url",
							MultiDigest: multiDigest,
							Version:     2,
						})
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("embedded error"))
					})
				})
			})
		})
	})
})
