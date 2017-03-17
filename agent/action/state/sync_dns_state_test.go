package state_test

import (
	"errors"

	. "github.com/cloudfoundry/bosh-agent/agent/action/state"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/system/fakes"
	fakeuuidgen "github.com/cloudfoundry/bosh-utils/uuid/fakes"
)

var _ = Describe("SyncDNSState", func() {
	var (
		localDNSState     LocalDNSState
		syncDNSState      SyncDNSState
		fakeFileSystem    *fakes.FakeFileSystem
		fakeUUIDGenerator *fakeuuidgen.FakeGenerator

		path string
		err  error
	)

	BeforeEach(func() {
		fakeFileSystem = fakes.NewFakeFileSystem()
		fakeUUIDGenerator = fakeuuidgen.NewFakeGenerator()
		path = "/blobstore-dns-records.json"
		syncDNSState = NewSyncDNSState(fakeFileSystem, path, fakeUUIDGenerator)
		err = nil
		localDNSState = LocalDNSState{}
	})

	Describe("#LoadState", func() {
		Context("when there is some failure loading", func() {
			Context("when SyncDNSState file cannot be read", func() {
				It("should fail loading DNS state", func() {
					_, err = syncDNSState.LoadState()
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("reading state file"))
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

				localDNSState, err := syncDNSState.LoadState()
				Expect(err).ToNot(HaveOccurred())
				Expect(localDNSState.Version).To(Equal(uint64(1234)))
			})
		})
	})

	Describe("#SaveState", func() {
		Context("when there are failures", func() {
			BeforeEach(func() {
				localDNSState = LocalDNSState{
					Version: 1234,
					Records: [][2]string{{"rec", "ip"}},
					VMKeys:  []string{"id", "instance-group", "az", "network", "deployment", "ip"},
					VMs:     [][]string{{"id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"}},
				}
			})

			Context("when saving the marshalled DNS state", func() {
				It("fails saving the DNS state", func() {
					fakeFileSystem.WriteFileError = errors.New("fake fail saving error")

					err = syncDNSState.SaveState(localDNSState)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("writing the blobstore DNS state"))
				})
			})

			It("should not override the existing records.json", func() {
				fakeUUIDGenerator.GeneratedUUID = "fake-generated-uuid"
				fakeFileSystem.WriteFileErrors[path+"fake-generated-uuid"] = errors.New("failed to write tmp file")

				err = syncDNSState.SaveState(localDNSState)
				Expect(err).To(MatchError("writing the blobstore DNS state: failed to write tmp file"))
			})

			Context("when generating a uuid fails", func() {
				It("returns an error", func() {
					fakeUUIDGenerator.GenerateError = errors.New("failed to generate a uuid")

					err = syncDNSState.SaveState(localDNSState)
					Expect(err).To(MatchError("generating uuid for temp file: failed to generate a uuid"))
				})
			})

			Context("when the rename fails", func() {
				It("returns an error", func() {
					fakeFileSystem.RenameError = errors.New("failed to rename")

					err = syncDNSState.SaveState(localDNSState)
					Expect(err).To(MatchError("renaming: failed to rename"))
				})
			})
		})

		Context("when there are no failures", func() {
			BeforeEach(func() {
				localDNSState = LocalDNSState{
					Version: 1234,
					Records: [][2]string{{"rec", "ip"}},
					VMKeys:  []string{"id", "instance-group", "az", "network", "deployment", "ip"},
					VMs:     [][]string{{"id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"}},
				}
				fakeUUIDGenerator.GeneratedUUID = "fake-generated-uuid"
			})

			It("saves the state in the path", func() {
				err = syncDNSState.SaveState(localDNSState)
				Expect(err).ToNot(HaveOccurred())

				Expect(fakeFileSystem.RenameOldPaths[0]).To(Equal(path + "fake-generated-uuid"))
				Expect(fakeFileSystem.RenameNewPaths[0]).To(Equal(path))

				contents, err := fakeFileSystem.ReadFile(path)
				Expect(err).ToNot(HaveOccurred())

				Expect(contents).To(MatchJSON(`{
					"version": 1234,
					"records": [
						["rec", "ip"]
					],
					"vm_keys": ["id", "instance-group", "az", "network", "deployment", "ip"],
					"vms": [
						["id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
					]
				}`))
			})
		})
	})

	Describe("#StateFileExists", func() {
		Context("when state file exists", func() {
			BeforeEach(func() {
				fakeFileSystem.WriteFile(path, []byte(`{"version":1}`))
			})

			It("returns true", func() {
				exists := syncDNSState.StateFileExists()
				Expect(exists).To(BeTrue())
			})
		})

		Context("when state file does not exist", func() {
			It("returns false", func() {
				exists := syncDNSState.StateFileExists()
				Expect(exists).To(BeFalse())
			})
		})
	})
})
