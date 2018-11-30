package tarpath_test

import (
	"archive/tar"
	"compress/gzip"
	"io/ioutil"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/agent/tarpath"
)

var _ = Describe("path prefix detection", func() {
	var (
		tmp      string
		detector *tarpath.PrefixDetector
	)

	BeforeEach(func() {
		var err error
		tmp, err = ioutil.TempDir("", "tarpath")
		Expect(err).NotTo(HaveOccurred())

		detector = tarpath.NewPrefixDetector()
	})

	AfterEach(func() {
		os.RemoveAll(tmp)
	})

	Context("when the files in the tarball do not have leading slashes", func() {
		It("returns false", func() {
			tgz := filepath.Join(tmp, "noleading.tgz")
			writeTgz(tgz, []string{
				"filepath/monit",
				"filepath/bin/ctl",
				"filepath/bin/drain",
				"filepath/.bosh/links.json",
				"otherpath/monit",
				"otherpath/bin/ctl",
				"otherpath/bin/drain",
				"otherpath/.bosh/links.json",
			})

			slash, err := detector.Detect(tgz, "filepath")
			Expect(err).NotTo(HaveOccurred())
			Expect(slash).To(BeFalse())

			slash, err = detector.Detect(tgz, "otherpath")
			Expect(err).NotTo(HaveOccurred())
			Expect(slash).To(BeFalse())
		})
	})

	Context("when the files in the tarball have leading slashes", func() {
		It("returns true", func() {
			tgz := filepath.Join(tmp, "noleading.tgz")
			writeTgz(tgz, []string{
				"./filepath/monit",
				"./filepath/monit",
				"./filepath/bin/ctl",
				"./filepath/bin/drain",
				"./filepath/.bosh/links.json",
				"./otherpath/monit",
				"./otherpath/bin/ctl",
				"./otherpath/bin/drain",
				"./otherpath/.bosh/links.json",
			})

			slash, err := detector.Detect(tgz, "filepath")
			Expect(err).NotTo(HaveOccurred())
			Expect(slash).To(BeTrue())

			slash, err = detector.Detect(tgz, "otherpath")
			Expect(err).NotTo(HaveOccurred())
			Expect(slash).To(BeTrue())
		})
	})

	Context("when the tarball does not have either of the paths we expect", func() {
		It("returns an error", func() {
			tgz := filepath.Join(tmp, "noleading.tgz")
			writeTgz(tgz, []string{
				"./wildcard/monit",
				"./wildcard/monit",
				"./wildcard/bin/ctl",
				"./wildcard/bin/drain",
				"./wildcard/.bosh/links.json",
			})

			_, err := detector.Detect(tgz, "filepath")
			Expect(err).To(MatchError("no file with prefix 'filepath' found"))
		})
	})
})

func writeTgz(path string, files []string) {
	f, err := os.Create(path)
	Expect(err).NotTo(HaveOccurred())
	defer f.Close()

	gw, err := gzip.NewWriterLevel(f, gzip.BestSpeed)
	Expect(err).NotTo(HaveOccurred())
	defer gw.Close()

	tw := tar.NewWriter(gw)
	defer tw.Close()

	for _, file := range files {
		tw.WriteHeader(&tar.Header{
			Name: file,
		})
	}
}
