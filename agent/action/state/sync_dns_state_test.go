package state_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/agent/action/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/system/fakes"
)

var _ = Describe("SyncDNSState", func() {
	var (
		blobstoreDNSState BlobstoreDNSState
		syncDNSState      SyncDNSState
		fakeFileSystem    *fakes.FakeFileSystem
		path              string
		err               error
	)

	BeforeEach(func() {
		fakeFileSystem = fakes.NewFakeFileSystem()
		path = "/blobstore-dns-records.json"
		syncDNSState = NewSyncDNSState(fakeFileSystem, path)
		err = nil
		blobstoreDNSState = BlobstoreDNSState{}
	})

	Describe("#LoadState", func() {
		Context("when there is some failure loading", func() {
			Context("when SyncDNSState file cannot be read", func() {
				It("should fail loading DNS state", func() {
					_, err = syncDNSState.LoadState()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("File not found"))
				})
			})

			Context("when SyncDNSState file cannot be unmarshalled", func() {
				Context("when state file is invalid JSON", func() {
					It("should fail loading DNS state", func() {
						fakeFileSystem.WriteFile(path, []byte("fake-state-file"))

						_, err := syncDNSState.LoadState()
						Expect(err).To(HaveOccurred())
						Expect(err.Error()).To(ContainSubstring("unmarshalling state file"))
					})
				})
			})
		})

		Context("when there are no failures", func() {
			It("loads and unmarshalls the DNS state with Version", func() {
				fakeFileSystem.WriteFile(path, []byte("{\"version\": 1234}"))

				blobstoreDNSState, err := syncDNSState.LoadState()
				Expect(err).ToNot(HaveOccurred())
				Expect(blobstoreDNSState.Version).To(Equal(uint32(1234)))
			})
		})
	})

	Describe("#SaveState", func() {
		Context("when there are failures", func() {
			BeforeEach(func() {
				blobstoreDNSState = BlobstoreDNSState{}
			})

			Context("when saving the marshalled DNS state", func() {
				It("fails saving the DNS state", func() {
					fakeFileSystem.WriteFileError = errors.New("fake fail saving error")

					err = syncDNSState.SaveState(blobstoreDNSState)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("writing the blobstore DNS state"))
				})
			})
		})

		Context("when there are no failures", func() {
			BeforeEach(func() {
				blobstoreDNSState = BlobstoreDNSState{
					Version: 1234,
				}
			})

			It("saves the state in the path", func() {
				err = syncDNSState.SaveState(blobstoreDNSState)
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
