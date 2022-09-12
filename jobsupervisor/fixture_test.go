package jobsupervisor_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

func readFixture(relativePath string) []byte {
	filePath, err := filepath.Abs(relativePath)
	Expect(err).ToNot(HaveOccurred())

	Expect(err).ToNot(HaveOccurred())
	content, err := os.ReadFile(filePath) //nolint:ineffassign,staticcheck

	return content
}
