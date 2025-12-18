package infrastructure_test

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system/fakes"

	. "github.com/cloudfoundry/bosh-agent/v2/infrastructure"
	"github.com/cloudfoundry/bosh-agent/v2/platform/platformfakes"
	boshsettings "github.com/cloudfoundry/bosh-agent/v2/settings"
)

var _ = Describe("CloudInitSettingsSource", func() {
	var (
		platform        *platformfakes.FakePlatform
		cmdRunner       *fakes.FakeCmdRunner
		source          *CloudInitSettingsSource
		settings        boshsettings.Settings
		encodedSettings string
	)

	BeforeEach(func() {
		platform = &platformfakes.FakePlatform{}
		cmdRunner = fakes.NewFakeCmdRunner()
		platform.GetRunnerReturns(cmdRunner)
		logger := logger.NewLogger(logger.LevelNone)
		source = NewCloudInitSettingsSource(platform, logger)
		settings = boshsettings.Settings{AgentID: "123"}
		settingsBytes, err := json.Marshal(settings)
		Expect(err).ToNot(HaveOccurred())
		encodedSettings = base64.StdEncoding.EncodeToString(settingsBytes)
	})

	Describe("Settings", func() {
		It("returns settings from vmware-rpctool", func() {
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: encodedSettings})

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("returns gzipped settings from vmware-rpctool", func() {
			var b bytes.Buffer
			w := gzip.NewWriter(&b)
			//nolint:errcheck
			defer w.Close()

			settingsBytes, err := json.Marshal(settings)
			Expect(err).ToNot(HaveOccurred())
			_, err = w.Write(settingsBytes)
			Expect(err).ToNot(HaveOccurred())
			err = w.Close()
			Expect(err).ToNot(HaveOccurred())

			gzippedEncodedSettings := base64.StdEncoding.EncodeToString(b.Bytes())

			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: gzippedEncodedSettings})

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))
		})

		It("returns settings from vmtoolsd when vmware-rpctool fails", func() {
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Error: errors.New("fail"), ExitStatus: 1})
			cmdRunner.AddCmdResult("vmtoolsd --cmd info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: encodedSettings})
			cmdRunner.AddCmdResult("vmware-rpctool info-set guestinfo.userdata ---", fakes.FakeCmdResult{Error: errors.New("fail"), ExitStatus: 1})
			cmdRunner.AddCmdResult("vmware-rpctool info-set guestinfo.userdata.encoding ", fakes.FakeCmdResult{Error: errors.New("fail"), ExitStatus: 1})

			settings, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())
			Expect(settings.AgentID).To(Equal("123"))
			Expect(cmdRunner.RunCommands).To(HaveLen(6))
			Expect(cmdRunner.RunCommands[0]).To(Equal([]string{"vmware-rpctool", "info-get guestinfo.userdata"}))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"vmtoolsd", "--cmd", "info-get guestinfo.userdata"}))
			Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"vmware-rpctool", "info-set guestinfo.userdata ---"}))
			Expect(cmdRunner.RunCommands[3]).To(Equal([]string{"vmtoolsd", "--cmd", "info-set guestinfo.userdata ---"}))
			Expect(cmdRunner.RunCommands[4]).To(Equal([]string{"vmware-rpctool", "info-set guestinfo.userdata.encoding "}))
			Expect(cmdRunner.RunCommands[5]).To(Equal([]string{"vmtoolsd", "--cmd", "info-set guestinfo.userdata.encoding "}))
		})

		It("returns an error if both tools fail", func() {
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Error: errors.New("fail"), ExitStatus: 1})
			cmdRunner.AddCmdResult("vmtoolsd --cmd info-get guestinfo.userdata", fakes.FakeCmdResult{Error: errors.New("fail"), ExitStatus: 1})

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("getting user data from vmware tools"))
		})

		It("returns an error if decoding fails", func() {
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: "not-base64"})

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("decoding user data"))
		})

		It("returns an error if unmarshalling fails", func() {
			encoded := base64.StdEncoding.EncodeToString([]byte("not-json"))
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: encoded})

			_, err := source.Settings()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Parsing settings from vmware tools"))
		})

		It("clears the guestinfo.userdata after reading it", func() {
			cmdRunner.AddCmdResult("vmware-rpctool info-get guestinfo.userdata", fakes.FakeCmdResult{Stdout: encodedSettings})
			cmdRunner.AddCmdResult("vmware-rpctool info-set guestinfo.userdata ---", fakes.FakeCmdResult{Stdout: ""})
			cmdRunner.AddCmdResult("vmware-rpctool info-set guestinfo.userdata.encoding ", fakes.FakeCmdResult{Stdout: ""})

			_, err := source.Settings()
			Expect(err).ToNot(HaveOccurred())

			Expect(cmdRunner.RunCommands).To(HaveLen(3))
			Expect(cmdRunner.RunCommands[1]).To(Equal([]string{"vmware-rpctool", "info-set guestinfo.userdata ---"}))
			Expect(cmdRunner.RunCommands[2]).To(Equal([]string{"vmware-rpctool", "info-set guestinfo.userdata.encoding "}))
		})
	})
})
