package integration_test

import (
    "fmt"

    biagentclient "github.com/cloudfoundry/bosh-agent/deployment/agentclient"
    bias "github.com/cloudfoundry/bosh-agent/deployment/applyspec"
    "github.com/cloudfoundry/bosh-agent/settings"

    . "github.com/onsi/ginkgo"
    . "github.com/onsi/gomega"
)

var _ = Describe("CertManager", func() {
    var agentClient biagentclient.AgentClient
    var (
        registrySettings settings.Settings
    )

    BeforeEach(func() {
        err := testEnvironment.StopAgent()
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.CleanupDataDir()
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.CleanupLogFile()
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.SetupConfigDrive()
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.UpdateAgentConfig("config-drive-agent.json")
        Expect(err).ToNot(HaveOccurred())

        networks, err := testEnvironment.GetVMNetworks()
        Expect(err).ToNot(HaveOccurred())

        registrySettings = settings.Settings{
            AgentID: "fake-agent-id",

            // note that this SETS the username and password for HTTP message bus access
            Mbus:    "https://mbus-user:mbus-pass@127.0.0.1:6868",

            Blobstore: settings.Blobstore{
                Type: "local",
                Options: map[string]interface{}{
                    "blobstore_path": "/var/vcap/data",
                },
            },
            Networks: networks,
        }

        err = testEnvironment.UpdateAgentConfig("root-partition-agent.json")
        Expect(err).ToNot(HaveOccurred())

        _, _, err = testEnvironment.AttachPartitionedRootDevice("/dev/sdz", 2048, 128)
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.StartRegistry(registrySettings)
        Expect(err).ToNot(HaveOccurred())

        err = testEnvironment.StartAgent()
        Expect(err).ToNot(HaveOccurred())

        agentClient, err = testEnvironment.StartAgentTunnel(registrySettings.Mbus)
        Expect(err).NotTo(HaveOccurred())
    })
    AfterEach(func() {
        err := testEnvironment.StopAgentTunnel()
        Expect(err).NotTo(HaveOccurred())

        err = testEnvironment.StopAgent()
        Expect(err).NotTo(HaveOccurred())
    })

    Context("on ubuntu", func() {
        It("adds and registers new certs on a fresh machine", func() {
            var cert string = "This certificate is the first one. It's more awesome than the other one.\n-----BEGIN CERTIFICATE-----\nMIIEJDCCAwygAwIBAgIJAO+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV\nBAYTAkNBMRMwEQYDVQQIEwpTb21lLVN0YXRlMSIBAgIJAO+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV\nBAYTAkNBMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX\naWRnaXRzIFB0eSBMdGQxIjAgBgNVBAMTGWR4MTkwLnRvci5waXZvdGFsbGFicy5EwHwYDVQQKExhJbnRlcm5ldCBX\naWRnaXRzIFB0eSBMdGQxIjAgBgNVBAMTGWR4MTkwLnRvci5waXZvdGFsbGFicy5j\nb20wHhcNMTUwNTEzMTM1NjA2WhcNMjUwNTEwMTM1NjA2WjBpMQswCQYDVQQGEwJD\nQTETMBEGA1UECBMKU29tZGackAF\nqokoSBXzJCJTt2P681gyqBDr/hUYzqpoXUsOTRisScbEbaSv8hTiTeFJUMyNQAqn\nDtmvI8bXKxU=\n-----END CERTIFICATE-----\n"
            settings := settings.Settings{Cert: cert}

            response, err := agentClient.UpdateSettings(settings)

            Expect(err).NotTo(HaveOccurred())
            fmt.Println("Response is:", response)
        })
    })

//    Context("on centos", func(){
//        BeforeEach(func() {
//
//        })
//
//        It("adds and registers new certs on a fresh machine", func() {
//
//        })
//
//    })

    Describe("Apply", func() {
        var (
            spec     bias.ApplySpec
        )

        BeforeEach(func() {
            spec = bias.ApplySpec{
                Deployment: "fake-deployment-name",
            }
        })

        Context("when agent responds with a value", func() {

            It("makes a POST request to the endpoint", func() {
                err := agentClient.Apply(spec)
                Expect(err).ToNot(HaveOccurred())
            })
        })
    })
})
