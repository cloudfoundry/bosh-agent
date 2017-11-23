// +build windows

package platform

import (
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

	var itAddsABlockingRule = func() {
		err := closeWinRMPort()
		Expect(err).ToNot(HaveOccurred())

		cmd := exec.Command("Powershell", "-Command", "Get-NetFirewallRule | where { $_.Action -eq \"Block\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq 5985 }")
		out, err := cmd.CombinedOutput()
		s := string(out)

		Expect(err).NotTo(HaveOccurred())
		Expect(s).ToNot(BeEmpty())
	}
	cmd := exec.Command("Powershell", "-Command", "Get-NetFirewallRule | where { $_.Action -eq \"Allow\" } | Get-NetFirewallPortFilter | where { $_.LocalPort -eq 5985 }")
	Context("Firewall rule allowing port 5985 exists", func() {

		BeforeEach(func() {
			deleteAllWinRMFirewallRules()

			err := setWinrmFirewall("allow")
			Expect(err).ToNot(HaveOccurred())
		})

		It("closes the port", func() {
			err := closeWinRMPort()
			Expect(err).ToNot(HaveOccurred())

			out, err := cmd.CombinedOutput()
			s := string(out)

			Expect(err).NotTo(HaveOccurred())
			Expect(s).To(BeEmpty())
		})

		It("adds a blocking rule", itAddsABlockingRule)
	})

	Context("Firewall rule allowing port 5985 does NOT exist", func() {
		BeforeEach(func() {
			deleteAllWinRMFirewallRules()
		})

		It("does not error", func() {
			err := closeWinRMPort()
			Expect(err).NotTo(HaveOccurred())
		})

		It("adds a blocking rule", itAddsABlockingRule)
	})
})
