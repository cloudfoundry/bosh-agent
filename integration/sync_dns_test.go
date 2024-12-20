package integration_test

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/v2/settings"

	boshcrypto "github.com/cloudfoundry/bosh-utils/crypto"
)

var _ = Describe("sync_dns", func() {
	var (
		fileSettings settings.Settings

		newRecordsVersion uint64
	)

	BeforeEach(func() {
		newRecordsVersion = uint64(time.Now().Unix())

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
					"blobstore_path": "/var/vcap/data/blobs",
				},
			},

			Disks: settings.Disks{
				Ephemeral: "/dev/sdh",
			},

			Networks: map[string]settings.Network{
				"default": settings.Network{
					UseDHCP: true,
					DNS:     []string{"8.8.8.8"},
				},
			},
		}

		err = testEnvironment.AttachDevice("/dev/sdh", 128, 2)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.CreateSettingsFile(fileSettings)
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	AfterEach(func() {
		err := testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	Context("sync_dns_with_signed_url action", func() {
		var (
			newRecordsJSONContent string
			blobDigest            boshcrypto.MultipleDigest
		)

		BeforeEach(func() {
			var err error

			err = testEnvironment.StartBlobstore()
			Expect(err).NotTo(HaveOccurred())

			newRecordsJSONContent = fmt.Sprintf(`{
				"version": %d,
				"records":[["216.58.194.206","google.com"],["54.164.223.71","pivotal.io"]],
				"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
				"record_infos": [
					["%d", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
				]
			}`, newRecordsVersion, newRecordsVersion)

			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo echo '%s' > /tmp/new-dns-records", newRecordsJSONContent))
			Expect(err).NotTo(HaveOccurred())

			shasum, err := testEnvironment.RunCommand("sudo shasum /tmp/new-dns-records | cut -f 1 -d ' '")
			Expect(err).NotTo(HaveOccurred())
			blobDigest = boshcrypto.MustNewMultipleDigest(boshcrypto.NewDigest(boshcrypto.DigestAlgorithmSHA1, strings.TrimSpace(shasum)))

			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo mv /tmp/new-dns-records %s", filepath.Join(testEnvironment.BlobstoreDir(), "records.json")))
			Expect(err).NotTo(HaveOccurred())
		})

		It("sends a sync_dns_with_signed_url message to the agent", func() {
			signedURL := "http://127.0.0.1:9091/get_package/records.json"
			response, err := testEnvironment.AgentClient.SyncDNSWithSignedURL(signedURL, blobDigest, newRecordsVersion)
			Expect(err).NotTo(HaveOccurred())
			Expect(response).To(Equal("synced"))

			newEtcHosts, err := testEnvironment.RunCommand("sudo cat /etc/hosts")
			Expect(err).NotTo(HaveOccurred())

			Expect(newEtcHosts).To(MatchRegexp("216.58.194.206\\s+google.com"))
			Expect(newEtcHosts).To(MatchRegexp("54.164.223.71\\s+pivotal.io"))

			instanceDNSRecords, err := testEnvironment.RunCommand("sudo cat /var/vcap/instance/dns/records.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(instanceDNSRecords).To(MatchJSON(newRecordsJSONContent))

			filePerms, err := testEnvironment.RunCommand("ls -l /var/vcap/instance/dns/records.json | cut -d ' ' -f 1,3,4")
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(filePerms)).To(Equal("-rw-r----- root vcap"))
		})
	})

	Context("sync_dns action", func() {
		It("sends a sync_dns message to agent", func() {
			newDNSRecords := settings.DNSRecords{
				Records: [][2]string{
					{"216.58.194.206", "google.com"},
					{"54.164.223.71", "pivotal.io"},
				},
			}
			contents, err := json.Marshal(newDNSRecords)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeNil())

			_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo touch /var/vcap/data/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo ls -la /var/vcap/data/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			recordsJSON := fmt.Sprintf(`{
		  "version": %d,
			"records":[["216.58.194.206","google.com"],["54.164.223.71","pivotal.io"]],
			"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
			"record_infos": [
				["id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
			]
		}`, newRecordsVersion)
			_, err = testEnvironment.RunCommand(fmt.Sprintf("sudo echo '%s' > /tmp/new-dns-records", recordsJSON))
			Expect(err).NotTo(HaveOccurred())

			blobDigest, err := testEnvironment.RunCommand("sudo shasum /tmp/new-dns-records | cut -f 1 -d ' '")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo mv /tmp/new-dns-records /var/vcap/data/blobs/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.AgentClient.SyncDNS("new-dns-records", strings.TrimSpace(blobDigest), newRecordsVersion)
			Expect(err).NotTo(HaveOccurred())

			newEtcHosts, err := testEnvironment.RunCommand("sudo cat /etc/hosts")
			Expect(err).NotTo(HaveOccurred())

			Expect(newEtcHosts).To(MatchRegexp("216.58.194.206\\s+google.com"))
			Expect(newEtcHosts).To(MatchRegexp("54.164.223.71\\s+pivotal.io"))

			instanceDNSRecords, err := testEnvironment.RunCommand("sudo cat /var/vcap/instance/dns/records.json")
			Expect(err).NotTo(HaveOccurred())
			Expect(instanceDNSRecords).To(MatchJSON(fmt.Sprintf(`{
			"version": %d,
			"records":[["216.58.194.206","google.com"],["54.164.223.71","pivotal.io"]],
			"record_keys": ["id", "instance_group", "az", "network", "deployment", "ip"],
			"record_infos": [
				["id-1", "instance-group-1", "az1", "network1", "deployment1", "ip1"]
			]
		}`, newRecordsVersion)))

			filePerms, err := testEnvironment.RunCommand("ls -l /var/vcap/instance/dns/records.json | cut -d ' ' -f 1,3,4")
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.TrimSpace(filePerms)).To(Equal("-rw-r----- root vcap"))
		})

		It("does not skip verification if no checksum is sent", func() {
			newDNSRecords := settings.DNSRecords{
				Records: [][2]string{
					{"216.58.194.206", "google.com"},
					{"54.164.223.71", "pivotal.io"},
				},
			}
			contents, err := json.Marshal(newDNSRecords)
			Expect(err).NotTo(HaveOccurred())
			Expect(contents).NotTo(BeNil())

			_, err = testEnvironment.RunCommand("sudo mkdir -p /var/vcap/data")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo touch /var/vcap/data/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo ls -la /var/vcap/data/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo echo '{\"records\":[[\"216.58.194.206\",\"google.com\"],[\"54.164.223.71\",\"pivotal.io\"]]}' > /tmp/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.RunCommand("sudo mv /tmp/new-dns-records /var/vcap/data/new-dns-records")
			Expect(err).NotTo(HaveOccurred())

			_, err = testEnvironment.AgentClient.SyncDNS("new-dns-records", "", 1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("No digest algorithm found. Supported algorithms: sha1, sha256, sha512"))
		})
	})
})
