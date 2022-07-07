package integration_test

import (
	"regexp"

	"github.com/cloudfoundry/bosh-agent/settings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

type ARPCache struct {
	MACAddr string
	State   string
}

var _ = Describe("DeleteARPEntries", func() {
	const (
		testMacAddress string = "12:34:56:78:9a:cd"
		testIP         string = "192.168.100.199"
	)

	var (
		fileSettings settings.Settings
	)

	var parseARPCacheIntoMap = func() (map[string]ARPCache, error) {
		cache := make(map[string]ARPCache)
		ARPResultsRegex := regexp.MustCompile(`([0-9.]+) dev [0-9a-z]+ (?:lladdr)? ([0-9:a-z]+)? ?([A-Z]+)`)
		lines, err := testEnvironment.RunCommand("ip neigh")
		if err != nil {
			return nil, err
		}

		for _, item := range ARPResultsRegex.FindAllStringSubmatch(lines, -1) {
			var ip, mac, state string

			ip = item[1]

			// When length is 3, then this IP address does not have an ARP entry.
			if len(item) == 3 {
				mac = ""
				state = item[2]
			} else {
				mac = item[2]
				state = item[3]
			}

			cache[ip] = ARPCache{MACAddr: mac, State: state}
		}

		return cache, nil
	}

	BeforeEach(func() {
		err := testEnvironment.CleanupLogFile()
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

		err = testEnvironment.CreateFilesettings(fileSettings)
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.SetUpDummyNetworkInterface(testIP, testMacAddress)
		Expect(err).ToNot(HaveOccurred())

		cache, _ := parseARPCacheIntoMap()
		macOfTestIP := cache[testIP].MACAddr
		Expect(macOfTestIP).To(Equal(testMacAddress))
	})

	AfterEach(func() {
		err := testEnvironment.TearDownDummyNetworkInterface()
		Expect(err).ToNot(HaveOccurred())

		err = testEnvironment.DetachDevice("/dev/sdh")
		Expect(err).ToNot(HaveOccurred())
	})

	JustBeforeEach(func() {
		err := testEnvironment.StartAgentTunnel()
		Expect(err).NotTo(HaveOccurred())
	})

	Context("on ubuntu", func() {
		It("deletes ARP entries from the cache", func() {
			err := testEnvironment.AgentClient.DeleteARPEntries([]string{testIP})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() ARPCache {
				ARPCache, _ := parseARPCacheIntoMap()
				return ARPCache[testIP]
			}, 10, 1).Should(Equal(ARPCache{}))
		})
	})
})
