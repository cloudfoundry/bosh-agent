package integration_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("version flag", func() {
	It("returns the version string and exits clean", func() {
		output, err := testEnvironment.RunCommand("sudo /var/vcap/bosh/bin/bosh-agent -v")
		Expect(err).ToNot(HaveOccurred())

		Expect(output).To(ContainSubstring("[DEV BUILD]"))
	})
})
