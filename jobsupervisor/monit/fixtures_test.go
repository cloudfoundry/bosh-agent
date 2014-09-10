package monit_test

import (
	"io/ioutil"
	"path/filepath"

	. "github.com/onsi/gomega"
)

const (
	statusWithMultipleServiceFixturePath = "../../Fixtures/monit_status_with_multiple_services.xml"
	statusFixturePath                    = "../../Fixtures/monit_status.xml"
)

func readFixture(relativePath string) []byte {
	filePath, err := filepath.Abs(relativePath)
	Expect(err).ToNot(HaveOccurred())

	content, err := ioutil.ReadFile(filePath)
	Expect(err).ToNot(HaveOccurred())

	return content
}
