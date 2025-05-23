package integration_test

import (
	"github.com/cloudfoundry/bosh-agent/v2/settings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("CertManager", func() {
	var (
		fileSettings settings.Settings
	)

	BeforeEach(func() {
		err := testEnvironment.CleanupDataDir()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CleanupLogFile()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.UpdateAgentConfig("file-settings-agent.json")
		Expect(err).ToNot(HaveOccurred())

		fileSettings = settings.Settings{
			Blobstore: settings.Blobstore{
				Type: "local",
				Options: map[string]interface{}{
					"blobstore_path": "/var/vcap/data",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("on ubuntu", func() {
		It("adds and registers new certs on a fresh machine", func() {
			var cert = `This certificate is the first one. It's more awesome than the other one.
-----BEGIN CERTIFICATE-----
MIIEJDCCAwygAwIBAgIJAO+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV
aWRnaXRzIFB0eSBMdGQxIjAgBgNVBAMTGWR4MTkwLnRvci5waXZvdGFsbGFicy5j
DtmvI8bXKxU=
-----END CERTIFICATE-----
Junk between the certs!
-----BEGIN CERTIFICATE-----
MIIEJDCCaWRnaXRzIFB0eSBMdGQxIjAgBgNVBAMTGWR4MTkwLnRvci5waXZvdGFs
b20wHhcNMTUwNTEzMTM1NjA2WhcNMjUwNTEwMTM1NjA2WjBpMQswCQYDVQQGEwJD
QTETMBEGA1U=
-----END CERTIFICATE-----`
			settings := settings.UpdateSettings{TrustedCerts: cert}

			err := testEnvironment.AgentClient.UpdateSettings(settings)

			Expect(err).NotTo(HaveOccurred())

			individualCerts, err := testEnvironment.RunCommand("ls /usr/local/share/ca-certificates/")
			Expect(err).NotTo(HaveOccurred())
			Expect(individualCerts).To(Equal("bosh-trusted-cert-1.crt\nbosh-trusted-cert-2.crt\n"))

			processedCerts, err := testEnvironment.RunCommand("grep MIIEJDCCAwygAwIBAgIJAO\\+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV /etc/ssl/certs/ca-certificates.crt")
			Expect(err).ToNot(HaveOccurred())
			Expect(processedCerts).To(Equal("MIIEJDCCAwygAwIBAgIJAO+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV\n"))
		})
	})
})
