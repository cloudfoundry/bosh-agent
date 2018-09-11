package disk_test

import (
	"github.com/cloudfoundry/bosh-agent/platform/windows/disk"
	"github.com/cloudfoundry/bosh-utils/system/fakes"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Manager", func() {
	var cmdRunner *fakes.FakeCmdRunner

	BeforeEach(func() {
		cmdRunner = new(fakes.FakeCmdRunner)
	})

	It("GetFormatter returns a Formatter", func() {
		manager := disk.NewWindowsDiskManager(cmdRunner)

		formatter := manager.GetFormatter()

		Expect(formatter).To(BeAssignableToTypeOf(&disk.Formatter{}))
	})

	It("GetLinker returns a Linker", func() {
		manager := disk.NewWindowsDiskManager(cmdRunner)

		linker := manager.GetLinker()

		Expect(linker).To(BeAssignableToTypeOf(&disk.Linker{}))
	})

	It("GetPartitioner returns a Partitioner", func() {
		manager := disk.NewWindowsDiskManager(cmdRunner)

		partitioner := manager.GetPartitioner()

		Expect(partitioner).To(BeAssignableToTypeOf(&disk.Partitioner{}))
	})
})
