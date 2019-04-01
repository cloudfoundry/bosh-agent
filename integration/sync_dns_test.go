package integration_test

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/gofrs/uuid"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agentclient"
	"github.com/cloudfoundry/bosh-agent/settings"
)

var _ = Describe("sync_dns", func() {
	var (
		agentClient      agentclient.AgentClient
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

		registrySettings = settings.Settings{
			AgentID: "fake-agent-id",

			// note that this SETS the username and password for HTTP message bus access
			Mbus: "https://mbus-user:mbus-pass@127.0.0.1:6868",

			Env: settings.Env{
				Bosh: settings.BoshEnv{
					TargetedBlobstores: settings.TargetedBlobstores{
						Packages: "custom-blobstore",
						Logs:     "custom-blobstore",
					},
					Blobstores: []settings.Blobstore{
						settings.Blobstore{
							Type: "local",
							Name: "ignored-blobstore",
							Options: map[string]interface{}{
								"blobstore_path": "/ignored/blobstore",
							},
						},
						settings.Blobstore{
							Type: "local",
							Name: "custom-blobstore",
							Options: map[string]interface{}{
								"blobstore_path": "/tmp/my-blobs",
							},
						},
					},
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartRegistry(registrySettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())

		agentClient, err = testEnvironment.StartAgentTunnel("mbus-user", "mbus-pass", 6868)
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.StopAgentTunnel()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())

		output, err := testEnvironment.RunCommand("sudo rm -rf /tmp/my-blobs")
		Expect(err).NotTo(HaveOccurred(), output)
	})

	It("sends a sync_dns message to agent", func() {
		blobID, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		instanceUUID, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		oldEtcHosts, err := testEnvironment.RunCommand("sudo cat /etc/hosts")
		Expect(err).NotTo(HaveOccurred())

		newDNSRecords := settings.DNSRecords{
			Records: [][2]string{
				{"216.58.194.206", "google.com"},
				{"54.164.223.71", "pivotal.io"},
			},
		}
		contents, err := json.Marshal(newDNSRecords)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).NotTo(BeNil())

		version := uint64(time.Now().Unix())
		recordsJSON := fmt.Sprintf(`{
		  "version": %d,
			"records":[["216.58.194.206","google.com"],["54.164.223.71","pivotal.io"]],
			"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
			"record_infos": [
				["%s", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
			]
		}`, version, instanceUUID.String())
		blobDigest, err := testEnvironment.CreateBlobFromStringInActualBlobstore(recordsJSON, "/tmp/my-blobs", blobID.String())
		Expect(err).NotTo(HaveOccurred())

		_, err = agentClient.SyncDNS(blobID.String(), strings.TrimSpace(blobDigest), version)
		Expect(err).NotTo(HaveOccurred())

		newEtcHosts, err := testEnvironment.RunCommand("sudo cat /etc/hosts")
		Expect(err).NotTo(HaveOccurred())

		Expect(newEtcHosts).To(MatchRegexp("216.58.194.206\\s+google.com"))
		Expect(newEtcHosts).To(MatchRegexp("54.164.223.71\\s+pivotal.io"))
		Expect(newEtcHosts).To(ContainSubstring(oldEtcHosts))

		instanceDNSRecords, err := testEnvironment.RunCommand("sudo cat /var/vcap/instance/dns/records.json")
		Expect(err).NotTo(HaveOccurred())
		Expect(instanceDNSRecords).To(MatchJSON(recordsJSON))

		filePerms, err := testEnvironment.RunCommand("ls -l /var/vcap/instance/dns/records.json | cut -d ' ' -f 1,3,4")
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.TrimSpace(filePerms)).To(Equal("-rw-r----- root vcap"))
	})

	It("does not skip verification if no checksum is sent", func() {
		blobID, err := uuid.NewV4()
		Expect(err).NotTo(HaveOccurred())

		version := uint64(time.Now().Unix())

		newDNSRecords := settings.DNSRecords{
			Records: [][2]string{
				{"216.58.194.206", "google.com"},
				{"54.164.223.71", "pivotal.io"},
			},
		}
		contents, err := json.Marshal(newDNSRecords)
		Expect(err).NotTo(HaveOccurred())
		Expect(contents).NotTo(BeNil())

		recordsJSON := fmt.Sprintf(`{
		  "version": %d,
			"records":[["216.58.194.206","google.com"],["54.164.223.71","pivotal.io"]],
		}`, version)
		_, err = testEnvironment.CreateBlobFromStringInActualBlobstore(recordsJSON, "/tmp/my-blobs", blobID.String())
		Expect(err).NotTo(HaveOccurred())

		_, err = agentClient.SyncDNS(blobID.String(), "", version)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("No digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
	})
})
