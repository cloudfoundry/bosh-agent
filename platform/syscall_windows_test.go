// +build windows

package platform_test

import (
	"fmt"
	"os/exec"

	. "github.com/cloudfoundry/bosh-agent/platform"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

const firewallRuleByActionAndPortTemplate = "Get-NetFirewallRule | where { $_.Action -eq \"%s\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq %d }"

var (
	executeErr error
	port       int
)

func testWinRMForPort(port int) func() {
	return func() {
		BeforeEach(func() {
			DeleteWinRMFirewallRule(port)
		})

		JustBeforeEach(func() {
			executeErr = CloseWinRMPort(port)
			Expect(executeErr).ToNot(HaveOccurred())
		})

		Context("firewall rule allowing inbound traffic on the port exists", func() {
			BeforeEach(func() {
				err := SetWinRMFirewall("allow", port)
				Expect(err).ToNot(HaveOccurred())
			})

			It("removes the firewall rule", func() {
				cmd := fmt.Sprintf(firewallRuleByActionAndPortTemplate, "Allow", port)
				out, err := exec.Command("Powershell", "-Command", cmd).CombinedOutput()

				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).To(BeEmpty())
			})

			It("creates a firewall rule to block inbound traffic on the port", func() {
				cmd := fmt.Sprintf(firewallRuleByActionAndPortTemplate, "Block", port)
				out, err := exec.Command("Powershell", "-Command", cmd).CombinedOutput()

				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).ToNot(BeEmpty())
			})
		})

		Context("firewall rule allowing inbound traffic on the port does NOT exist", func() {
			It("creates a firewall rule to block inbound traffic on the port", func() {
				cmd := fmt.Sprintf(firewallRuleByActionAndPortTemplate, "Block", port)
				out, err := exec.Command("Powershell", "-Command", cmd).CombinedOutput()

				Expect(err).NotTo(HaveOccurred())
				Expect(string(out)).ToNot(BeEmpty())
			})
		})
	}
}

var _ = FDescribe("CloseWinRMPort", func() {
	Context("test port 5985", testWinRMForPort(5985))
	Context("test port 5986", testWinRMForPort(5986))

	Context("when the port is invalid", func() {
		JustBeforeEach(func() {
			executeErr = CloseWinRMPort(port)
		})

		Context("and the port is -1", func() {
			BeforeEach(func() {
				port = -1
			})

			It("returns an error", func() {
				Expect(executeErr.Error()).To(MatchRegexp(`\(.+\): .+`))
			})
		})

		Context("and the port is too large", func() {
			BeforeEach(func() {
				port = 65536
			})

			It("returns an error", func() {
				Expect(executeErr.Error()).To(MatchRegexp(`\(.+\): .+`))
			})
		})
	})
})
