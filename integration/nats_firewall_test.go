package integration_test

import (
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/format"
)

var _ = Describe("nats firewall", func() {
	BeforeEach(func() {
	})

	It("sets up the outgoing nats firewall", func() {
		format.MaxLength = 0
		//restore original settings of bosh from initial deploy of this VM.
		testEnvironment.RunCommand("sudo cp /settings-backup/*.json /var/vcap/bosh/")
		err := testEnvironment.StartAgent()
		Expect(err).ToNot(HaveOccurred())
		//Wait a maximum of 300 seconds
		Eventually(func() string {
			logs, _ := testEnvironment.RunCommand("sudo cat /var/vcap/bosh/log/current")
			return logs
		}, 300).Should(ContainSubstring("UbuntuNetManager"))

		output, err := testEnvironment.RunCommand("sudo iptables -t mangle -L")
		Expect(err).To(BeNil())
		//Check iptables for inclusion of the nats_cgroup_id
		Expect(output).To(MatchRegexp("ACCEPT *tcp  --  anywhere.*tcp dpt:4222 cgroup 2958295042"))
		Expect(output).To(MatchRegexp("DROP *tcp  --  anywhere.*tcp dpt:4222"))

		boshEnv := os.Getenv("BOSH_ENVIRONMENT")

		//check that we cannot access the director nats, -w2 == timeout 2 seconds
		out, err := testEnvironment.RunCommand(fmt.Sprintf("nc %v 4222 -w2 -v", boshEnv))
		Expect(err).NotTo(BeNil())
		Expect(out).To(ContainSubstring("port 4222 (tcp) timed out"))

		out, err = testEnvironment.RunCommand(fmt.Sprintf(`sudo sh -c '
            echo $$ >> $(cat /proc/self/mounts | grep ^cgroup | grep net_cls | cut -f2 -d" ")/nats-api-access/tasks
            nc %v 4222 -w2 -v'
		`, boshEnv))
		Expect(out).To(MatchRegexp("INFO.*server_id.*version.*host.*"))
		Expect(err).To(BeNil())

	})

	AfterEach(func() {
		err := testEnvironment.StopAgent()
		Expect(err).NotTo(HaveOccurred())

	})

})
