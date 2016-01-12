package app

import (
	"errors"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

var (
	fs *fakesys.FakeFileSystem
	agentStateJSONPath string
)

var _ = Describe("SaveState", func() {
	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		agentStateJSONPath = "/agent_state.json"
	})

	It("saves the state file with the appropriate properties", func() {
		SaveState(fs, agentStateJSONPath, State{HostsConfigured: true, HostnameConfigured: true})

		state, err := LoadState(fs, agentStateJSONPath)

		Expect(err).ToNot(HaveOccurred())
		Expect(state.HostsConfigured).To(BeTrue())
		Expect(state.HostnameConfigured).To(BeTrue())
	})

	It("saves the state file with the properties passed in", func() {
		SaveState(fs, agentStateJSONPath, State{HostsConfigured: true})

		state, err := LoadState(fs, agentStateJSONPath)

		Expect(err).ToNot(HaveOccurred())
		Expect(state.HostsConfigured).To(BeTrue())
		Expect(state.HostnameConfigured).To(BeFalse())
	})

	It("returns an error when it can't write the file", func() {
		fs.WriteFileError = errors.New("ENXIO: disk failed")
		err := SaveState(fs, "/asdf", State{HostsConfigured: true})

		Expect(err.Error()).To(ContainSubstring("disk failed"))
	})

	It("returns an error when it tries to save a nil object", func() {
		err := SaveState(fs, agentStateJSONPath, State{})

		Expect(err).ToNot(HaveOccurred())
	})
})

var _ = Describe("LoadState", func() {
	BeforeEach(func() {
		fs = fakesys.NewFakeFileSystem()
		agentStateJSONPath = "/agent_state.json"
	})

	Context("When the agent's state file cannot be found", func() {
		It("returns state object with false properties", func() {
			state, err := LoadState(fs, "/non-existent/agent_state.json")

			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(state.HostsConfigured).To(BeFalse())
			Expect(state.HostnameConfigured).To(BeFalse())
		})
	})

	Context("When the agent's state is ''", func() {
		It("returns an error and a state object with false properties", func() {
			state, err := LoadState(fs, "")

			Expect(err.Error()).To(ContainSubstring("not found"))
			Expect(state.HostsConfigured).To(BeFalse())
			Expect(state.HostnameConfigured).To(BeFalse())
		})
	})

	Context("When the agent cannot read the state file due to a failed disk", func() {
		It("returns an error and a state object with false properties", func() {
			fs.WriteFileString(agentStateJSONPath, `{
				"hosts_configured": true,
				"hostname_configured": true
			}`)

			fs.RegisterReadFileError(agentStateJSONPath, errors.New("ENXIO: disk failed"))

			state, err := LoadState(fs, agentStateJSONPath)

			Expect(err.Error()).To(ContainSubstring("disk failed"))
			Expect(state.HostsConfigured).To(BeFalse())
			Expect(state.HostnameConfigured).To(BeFalse())
		})
	})

	Context("When the agent cannot parse the state file due to malformed JSON", func() {
		It("returns an error and a state object with false properties", func() {
			fs.WriteFileString(agentStateJSONPath, "malformed-JSON")

			state, err := LoadState(fs, agentStateJSONPath)

			Expect(err).To(HaveOccurred())
			Expect(state.HostsConfigured).To(BeFalse())
			Expect(state.HostnameConfigured).To(BeFalse())
		})
	})
})
