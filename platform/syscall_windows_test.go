// +build windows

package platform

import (
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"sync"

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
	DeleteLocalUser = deleteLocalUser
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

var _ = Describe("Windows Syscalls and Helper functions", func() {
	It("Generates valid Windows passwords", func() {
		// 100,000 iterations takes about 140ms to run in a VM.
		for i := 0; i < 100000; i++ {
			s, err := RandomPassword()
			Expect(err).To(BeNil())
			Expect(s).To(HaveLen(14))
			Expect(s).ToNot(ContainSubstring("/"))
			Expect(ValidWindowsPassword(s)).To(BeTrue())
		}
	})

	expectedUserNames := func() ([]string, error) {
		cmd := exec.Command("PowerShell", "-Command",
			"Get-WmiObject -Class Win32_UserAccount | foreach { $_.Name }")

		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, err
		}
		exp := strings.Fields(string(out))
		sort.Strings(exp)
		return exp, nil
	}

	It("Lists local user accounts", func() {
		exp, err := expectedUserNames()
		Expect(err).To(Succeed())

		names, err := LocalAccountNames()
		Expect(err).To(Succeed())

		sort.Strings(names)
		Expect(names).To(Equal(exp))
	})

	It("Does not fail in a tight loop", func() {
		var wg sync.WaitGroup
		numCPU := runtime.NumCPU()
		if numCPU > 4 {
			numCPU = 4
		}
		for i := 0; i < numCPU; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for i := 0; i < 5000; i++ {
					names, err := LocalAccountNames()
					Expect(err).To(Succeed())
					Expect(names).ToNot(HaveLen(0))
				}
			}()
		}
		wg.Wait()
	})
})
