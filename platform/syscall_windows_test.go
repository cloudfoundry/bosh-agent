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

// SetSSHEnabled sets the function called by GetHostPublicKey to determine if
// ssh is enabled.
func SetSSHEnabled(new func() error) (previous func() error) {
	previous = sshEnabled
	sshEnabled = new
	return previous
}

func SetAdministratorUserName(name string) (previous string) {
	previous = administratorUserName
	administratorUserName = name
	return previous
}

var _ = Describe("closeWinRMPort", func() {

	var itAddsABlockingRule = func(port int) {
		err := closeWinRMPort(port)
		Expect(err).ToNot(HaveOccurred())

		cmd := fmt.Sprintf("Get-NetFirewallRule | where { $_.Action -eq \"Block\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq %d }", port)
		out, err := exec.Command("Powershell", "-Command", cmd).CombinedOutput()
		s := string(out)

		Expect(err).NotTo(HaveOccurred())
		Expect(s).ToNot(BeEmpty())
	}

	var testWinRMForPort = func(port int) {

		Context(fmt.Sprintf("for port %d", port), func() {

			BeforeEach(func() {
				deleteWinRMFirewallRule(port)
			})

			JustBeforeEach(func() {
				err := closeWinRMPort(port)
				Expect(err).ToNot(HaveOccurred())
			})

			Context("firewall rule allowing inbound traffic on the port exists", func() {

				BeforeEach(func() {
					err := setWinrmFirewall("allow", port)
					Expect(err).ToNot(HaveOccurred())
				})

				It("removes the firewall rule", func() {
					cmd := fmt.Sprintf("Get-NetFirewallRule | where { $_.Action -eq \"Allow\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq %d }", port)
					out, err := exec.Command("Powershell", "-Command", cmd).CombinedOutput()
					s := string(out)

					Expect(err).NotTo(HaveOccurred())
					Expect(s).To(BeEmpty())
				})

				It("creates a firewall rule to block inbound traffic on the port", func() {
					itAddsABlockingRule(port)
				})
			})

			Context("firewall rule allowing inbound traffic on the port does NOT exist", func() {
				It("creates a firewall rule to block inbound traffic on the port", func() {
					itAddsABlockingRule(port)
				})
			})
		})
	}

	testWinRMForPort(5985)
	testWinRMForPort(5986)
})
