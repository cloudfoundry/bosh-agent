package net

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SetupFirewall Linux", func() {
	// covers the case for http_metadata_service where on some IaaSs we cannot yet know the contents of
	// agent-settings.json since http_metadata_service is responsible for pulling the data.
	When("mbus url is empty", func() {
		It("returns early without an error", func() {
			err := SetupNatsFirewall("")
			Expect(err).ToNot(HaveOccurred())
		})
	})
	// create no rule on a create-env
	When("mbus url starts with https://", func() {
		It("returns early without an error", func() {
			err := SetupNatsFirewall("https://")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
