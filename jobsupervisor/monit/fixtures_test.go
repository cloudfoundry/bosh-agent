package monit_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/gomega"
)

const (
	statusWithMultipleServiceFixturePath = "test_assets/monit_status_multiple.xml"
	statusFixturePath                    = "test_assets/monit_status.xml"
)

func readFixture(relativePath string) []byte {
	filePath, err := filepath.Abs(relativePath)
	Expect(err).ToNot(HaveOccurred())

	content, err := os.ReadFile(filePath)
	Expect(err).ToNot(HaveOccurred())

	return content
}
