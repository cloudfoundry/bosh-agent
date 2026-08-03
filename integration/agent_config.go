package integration

import (
	"github.com/cloudfoundry/bosh-agent/v2/app"
	boshinf "github.com/cloudfoundry/bosh-agent/v2/infrastructure"
	boshplatform "github.com/cloudfoundry/bosh-agent/v2/platform"
)

var DefaultAgentConfig = app.Config{
	Platform: boshplatform.Options{
		Linux: boshplatform.LinuxOptions{
			UseDefaultTmpDir:              true,
			UsePreformattedPersistentDisk: true,
			BindMountPersistentDisk:       false,
			DevicePathResolutionType:      "virtio",
		},
	},
	Infrastructure: boshinf.Options{
		Settings: boshinf.SettingsOptions{
			Sources: boshinf.SourceOptionsSlice{
				boshinf.FileSourceOptions{SettingsPath: "/var/vcap/settings.json"},
			},
		},
	},
}
