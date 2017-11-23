// +build windows

package platform

import (
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var (
	// Export for testing
	UserHomeDirectory    = userHomeDirectory
	RandomPassword       = randomPassword
	ValidWindowsPassword = validPassword
	LocalAccountNames    = localAccountNames

	// Export for test cleanup
	DeleteUserProfile = deleteUserProfile
)

const firewallRuleByActionAndPortTemplate = "Get-NetFirewallRule | where { $_.Action -eq \"%s\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq %d }"

func testWinRMForPort(port int) func() {
	return func() {
		BeforeEach(func() {
			deleteWinRMFirewallRule(port)
		})

		JustBeforeEach(func() {
			executeErr := closeWinRMPort(port)
			Expect(executeErr).ToNot(HaveOccurred())
		})

		Context("firewall rule allowing inbound traffic on the port exists", func() {
			BeforeEach(func() {
				err := setWinrmFirewall("allow", port)
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

var _ = Describe("closeWinRMPort", func() {
	Context("test port 5985", testWinRMForPort(5985))
	Context("test port 5986", testWinRMForPort(5986))
})
