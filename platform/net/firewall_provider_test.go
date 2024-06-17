package net

import (
	"fmt"

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

var _ = Describe("Nftables Rules", func() {
	Describe("Parsing and Validating Mbus URL", func() {
		Context("With different types of mbus URLs", func() {
			It("Should handle valid and invalid URLs", func() {
				tests := []struct {
					mbus       string
					shouldFail bool
				}{
					{"http://valid.url:4222", false},
					{"https://valid.url:4222", false},
					{"invalid-url", true},
					{"", false},
				}

				for _, test := range tests {
					err := SetupNatsFirewall(test.mbus)
					if test.shouldFail {
						Expect(err).To(HaveOccurred(), fmt.Sprintf("Expected error for mbus: %s", test.mbus))
					} else {
						Expect(err).ToNot(HaveOccurred(), fmt.Sprintf("Did not expect error for mbus: %s", test.mbus))
					}
				}
			})
		})
	})

	Describe("Adding Nftables Rules", func() {
		Context("With different rule expressions", func() {
			FIt("Should add rules for cgroup match", func() {
				// this tests needs the bosh-agent cgroup to be created and nftables to be installed
				err := SetupNFTables("1.2.3.4", "1234")
				Expect(err).ToNot(HaveOccurred(), "Failed to setup nftables")
				// results of running this should create the following rules
				// socket cgroupv2 level 2 "system.slice/bosh-agent.service" ip daddr 1.2.3.4 tcp dport 1234 log prefix "Matched cgroup bosh-agent nats rule: " accept
				// meta skuid 0 ip daddr 1.2.3.4tcp dport 1234 log prefix "Matched skuid director nats rule: " accept
				// ip daddr 1.2.3.4 tcp dport 1234 log prefix "dropped nats rule: " drop

			})
		})
	})
})
