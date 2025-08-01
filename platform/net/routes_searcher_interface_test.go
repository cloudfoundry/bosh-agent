package net_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/platform/net"
	boship "github.com/cloudfoundry/bosh-agent/v2/platform/net/ip"
)

var _ = Describe("Route", func() {
	Describe("IsDefault", func() {
		It("returns true if destination is 0.0.0.0", func() {
			Expect(Route{Destination: "0.0.0.0"}.IsDefault(boship.IPv4)).To(BeTrue())
		})

		It("returns true if destination is ::", func() {
			Expect(Route{Destination: "::"}.IsDefault(boship.IPv6)).To(BeTrue())
		})

		It("returns false if destination is not ::", func() {
			Expect(Route{Destination: "::1"}.IsDefault(boship.IPv6)).To(BeFalse())
		})

		It("returns false if destination is not 0.0.0.0", func() {
			Expect(Route{}.IsDefault(boship.IPv4)).To(BeFalse())
			Expect(Route{Destination: "1.1.1.1"}.IsDefault(boship.IPv4)).To(BeFalse())
		})
	})
})
