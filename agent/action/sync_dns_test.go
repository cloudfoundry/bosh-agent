package action_test

import (
	"encoding/json"
	"errors"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/agent/action"

	"github.com/cloudfoundry/bosh-agent/agent/action/state"

	boshsettings "github.com/cloudfoundry/bosh-agent/settings"

	fakelogger "github.com/cloudfoundry/bosh-agent/logger/fakes"
	fakeplatform "github.com/cloudfoundry/bosh-agent/platform/fakes"
	fakesettings "github.com/cloudfoundry/bosh-agent/settings/fakes"
	fakeblobstore "github.com/cloudfoundry/bosh-utils/blobstore/fakes"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("SyncDNS", func() {
	var (
		action              SyncDNS
		fakeBlobstore       *fakeblobstore.FakeBlobstore
		fakeSettingsService *fakesettings.FakeSettingsService
		fakePlatform        *fakeplatform.FakePlatform
		fakeFileSystem      *fakesys.FakeFileSystem
		logger              *fakelogger.FakeLogger
	)

	BeforeEach(func() {
		logger = &fakelogger.FakeLogger{}
		fakeBlobstore = fakeblobstore.NewFakeBlobstore()
		fakeSettingsService = &fakesettings.FakeSettingsService{}
		fakePlatform = fakeplatform.NewFakePlatform()
		fakeFileSystem = fakePlatform.GetFs().(*fakesys.FakeFileSystem)

		action = NewSyncDNS(fakeBlobstore, fakeSettingsService, fakePlatform, logger)
	})

	AssertActionIsNotAsynchronous(action)
	AssertActionIsNotPersistent(action)
	AssertActionIsLoggable(action)

	AssertActionIsNotResumable(action)
	AssertActionIsNotCancelable(action)

	Context("#Run", func() {
		var stateFilePath string

		BeforeEach(func() {
			fakeDNSRecordsString := `
							{
								"version": 2,
								"records": [
									["fake-ip0", "fake-name0"],
									["fake-ip1", "fake-name1"]
								]
							}`

			err := fakeFileSystem.WriteFileString("fake-blobstore-file-path", fakeDNSRecordsString)
			Expect(err).ToNot(HaveOccurred())

			fakeBlobstore.GetFileName = "fake-blobstore-file-path"
			stateFilePath = filepath.Join(fakePlatform.GetDirProvider().BaseDir(), "local_dns_state.json")
		})

		Context("when local DNS state version is >= Run's version", func() {
			BeforeEach(func() {
				fakeBlobstore.GetError = errors.New("fake-blobstore-get-error")
			})

			Context("when the version equals the Run's version", func() {
				BeforeEach(func() {
					err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 2}`)
					Expect(err).ToNot(HaveOccurred())

					fakeFileSystem.WriteFileError = errors.New("fake-write-error")
				})

				It("returns with no error and does no writes and no gets to blobstore", func() {
					_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
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
					_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())
				})
			})
		})

		Context("when local DNS state version is < Run's version", func() {
			BeforeEach(func() {
				err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 1}`)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("when blobstore contains DNS records", func() {
				It("accesses the blobstore and fetches DNS records", func() {
					response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(fakeBlobstore.GetBlobIDs).To(ContainElement("fake-blobstore-id"))
					Expect(fakeBlobstore.GetFingerprints).To(ContainElement("fake-fingerprint"))

					Expect(fakeBlobstore.GetError).ToNot(HaveOccurred())
					Expect(fakeBlobstore.GetFileName).ToNot(Equal(""))
				})

				It("reads the DNS records from the blobstore file", func() {
					response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(fakeBlobstore.GetError).ToNot(HaveOccurred())
					Expect(fakeBlobstore.GetFileName).To(Equal("fake-blobstore-file-path"))
					Expect(fakeFileSystem.ReadFileError).ToNot(HaveOccurred())
				})

				It("fails reading the DNS records from the blobstore file", func() {
					fakeFileSystem.RegisterReadFileError("fake-blobstore-file-path", errors.New("fake-error"))

					response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).To(HaveOccurred())
					Expect(response).To(Equal(""))
					Expect(err.Error()).To(ContainSubstring("reading fake-blobstore-file-path from blobstore"))
					Expect(fakeFileSystem.FileExists("fake-blobstore-file-path")).To(BeFalse())
				})

				It("deletes the file once read", func() {
					_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())

					Expect(fakeFileSystem.FileExists("fake-blobstore-file-path")).To(BeFalse())
				})

				It("logs when the dns blob file can't be deleted", func() {
					fakeFileSystem.RemoveAllStub = func(path string) error {
						if path == "fake-blobstore-file-path" {
							return errors.New("fake-file-path-error")
						}
						return nil
					}
					_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())

					tag, message, _ := logger.ErrorArgsForCall(0)
					Expect(tag).To(Equal("Sync DNS action"))
					Expect(message).To(Equal("Failed to remove dns blob file at path 'fake-blobstore-file-path'"))
				})

				It("saves DNS records to the platform", func() {
					response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
					Expect(err).ToNot(HaveOccurred())
					Expect(response).To(Equal("synced"))

					Expect(fakePlatform.SaveDNSRecordsError).To(BeNil())
					Expect(fakePlatform.SaveDNSRecordsDNSRecords).To(Equal(boshsettings.DNSRecords{
						Version: 2,
						Records: [][2]string{
							{"fake-ip0", "fake-name0"},
							{"fake-ip1", "fake-name1"},
						},
					}))
				})

				Context("local DNS state operations", func() {
					Context("when loading succeeds", func() {
						Context("when saving succeeds", func() {
							It("saves the new local DNS state", func() {
								response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 3)
								Expect(err).ToNot(HaveOccurred())
								Expect(response).To(Equal("synced"))

								contents, err := fakeFileSystem.ReadFile(stateFilePath)
								Expect(err).ToNot(HaveOccurred())
								localDNSState := state.LocalDNSState{}
								err = json.Unmarshal(contents, &localDNSState)
								Expect(err).ToNot(HaveOccurred())
								Expect(localDNSState).To(Equal(state.LocalDNSState{Version: 3}))
							})
						})

						Context("when loading fails", func() {
							BeforeEach(func() {
								fakeFileSystem.RegisterReadFileError(stateFilePath, errors.New("fake-read-error"))
							})

							It("returns an error", func() {
								_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 3)
								Expect(err).To(HaveOccurred())
								Expect(err.Error()).To(ContainSubstring("loading local DNS state"))
							})
						})
					})

					Context("when saving fails", func() {
						BeforeEach(func() {
							fakeFileSystem.WriteFileError = errors.New("fake-write-error")
						})

						It("returns an error", func() {
							_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 3)
							Expect(err).To(HaveOccurred())
							Expect(err.Error()).To(ContainSubstring("saving local DNS state"))
						})
					})
				})

				Context("when DNS records is invalid", func() {
					BeforeEach(func() {
						err := fakeFileSystem.WriteFileString("fake-blobstore-file-path", "")
						Expect(err).ToNot(HaveOccurred())
					})

					It("fails unmarshalling the DNS records from the file", func() {
						_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("unmarshalling DNS records"))
					})
				})

				Context("when platform fails to save DNS records", func() {
					BeforeEach(func() {
						fakePlatform.SaveDNSRecordsError = errors.New("fake-error")
					})

					It("fails to save DNS records on the platform", func() {
						_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("saving DNS records"))
					})
				})
			})

			Context("when blobstore does not contain DNS records", func() {
				BeforeEach(func() {
					fakeBlobstore.GetFileName = "fake-blobstore-file-path-does-not-exist"
				})

				Context("when blobstore returns an error", func() {
					It("fails with an wrapped error", func() {
						_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("reading fake-blobstore-file-path-does-not-exist from blobstore"))
					})
				})

				Context("when blobstore returns a file that cannot be read", func() {
					BeforeEach(func() {
						fakeBlobstore.GetFileName = "fake-blobstore-file-path"
						fakeBlobstore.GetError = nil

						fakeFileSystem.RemoveAllStub = func(path string) error {
							if path == "fake-blobstore-file-path" {
								return errors.New("fake-remove-all-error")
							}
							return nil
						}
					})

					Context("when file removal failed", func() {
						It("logs error", func() {
							_, _ = action.Run("fake-blobstore-id", "fake-fingerprint", 2)
							tag, message, _ := logger.ErrorArgsForCall(0)
							Expect(tag).To(Equal("Sync DNS action"))
							Expect(message).To(Equal("Failed to remove dns blob file at path 'fake-blobstore-file-path'"))
						})
					})
				})
			})
		})

		Context("when local DNS state could not be read", func() {
			BeforeEach(func() {
				err := fakeFileSystem.WriteFileString(stateFilePath, `{"version": 2}`)
				Expect(err).ToNot(HaveOccurred())

				fakeFileSystem.ReadFileError = errors.New("fake-read-error")
				fakeFileSystem.RegisterReadFileError(stateFilePath, fakeFileSystem.ReadFileError)
			})

			It("returns error", func() {
				_, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("reading local DNS state"))
			})
		})

		Context("when there is no local DNS state", func() {
			It("runs successfully and creates a new state file", func() {
				Expect(fakeFileSystem.FileExists(stateFilePath)).To(BeFalse())

				response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 1)
				Expect(err).ToNot(HaveOccurred())
				Expect(response).To(Equal("synced"))

				contents, err := fakeFileSystem.ReadFile(stateFilePath)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(contents)).To(Equal(`{"version":1}`))
			})

			It("saves DNS records to the platform", func() {
				response, err := action.Run("fake-blobstore-id", "fake-fingerprint", 2)
				Expect(err).ToNot(HaveOccurred())
				Expect(response).To(Equal("synced"))

				Expect(fakePlatform.SaveDNSRecordsError).To(BeNil())
				Expect(fakePlatform.SaveDNSRecordsDNSRecords).To(Equal(boshsettings.DNSRecords{
					Version: 2,
					Records: [][2]string{
						{"fake-ip0", "fake-name0"},
						{"fake-ip1", "fake-name1"},
					},
				}))
			})
		})
	})
})
