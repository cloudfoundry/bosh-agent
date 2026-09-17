package platform

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("mappedDevicePathResolverTimeout", func() {
	It("uses the configured timeout when VirtioDevicePathResolverTimeout is set", func() {
		timeout := mappedDevicePathResolverTimeout(LinuxOptions{VirtioDevicePathResolverTimeout: 90})
		Expect(timeout).To(Equal(90 * time.Second))
	})

	It("falls back to the default timeout when VirtioDevicePathResolverTimeout is unset", func() {
		timeout := mappedDevicePathResolverTimeout(LinuxOptions{})
		Expect(timeout).To(Equal(defaultMappedDiskWaitTimeout))
	})

	It("falls back to the default timeout when VirtioDevicePathResolverTimeout is negative", func() {
		timeout := mappedDevicePathResolverTimeout(LinuxOptions{VirtioDevicePathResolverTimeout: -1})
		Expect(timeout).To(Equal(defaultMappedDiskWaitTimeout))
	})

	It("defaults to 60 seconds", func() {
		Expect(defaultMappedDiskWaitTimeout).To(Equal(60 * time.Second))
	})
})
