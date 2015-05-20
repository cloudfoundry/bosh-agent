package cert_test

import (
	"errors"
	"fmt"
	"os"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/cloudfoundry/bosh-agent/platform/cert"
	"github.com/cloudfoundry/bosh-utils/logger"
	"github.com/cloudfoundry/bosh-utils/system"
	fakesys "github.com/cloudfoundry/bosh-utils/system/fakes"
)

const cert1 string = `-----BEGIN CERTIFICATE-----
MIIEJDCCAwygAwIBAgIJAO+CqgiJnCgpMA0GCSqGSIb3DQEBBQUAMGkxCzAJBgNV
BAYTAkNBMRMwEQYDVQQIEwpTb21lLVN0YXRlMSEwHwYDVQQKExhJbnRlcm5ldCBX
qokoSBXzJCJTt2P681gyqBDr/hUYzqpoXUsOTRisScbEbaSv8hTiTeFJUMyNQAqn
DtmvI8bXKxU=
-----END CERTIFICATE-----`

var _ = Describe("Certificate Manager", func() {
	var log logger.Logger
	BeforeEach(func() {
		log = logger.NewLogger(logger.LevelNone)
	})

	Describe("CertificateSplitting", func() {
		It("splits 2 back-to-back certificates", func() {
			certs := fmt.Sprintf("%s\n%s\n", cert1, cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(result[1]).To(Equal(cert1))
			Expect(len(result)).To(Equal(2))
		})

		It("splits 2 back-to-back certificates without trailing newline", func() {
			certs := fmt.Sprintf("%s\n%s", cert1, cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(result[1]).To(Equal(cert1))
			Expect(len(result)).To(Equal(2))
		})

		It("splits 2 back-to-back certificates ignoring junk between them", func() {
			certs := fmt.Sprintf("%s\n abcdefghij %s\n", cert1, cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(result[1]).To(Equal(cert1))
			Expect(len(result)).To(Equal(2))
		})

		It("handles 1 certificate with trailing newline", func() {
			certs := fmt.Sprintf("%s\n", cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(len(result)).To(Equal(1))
		})

		It("handles 1 certificate without trailing newline", func() {
			result := cert.SplitCerts(cert1)
			Expect(result[0]).To(Equal(cert1))
			Expect(len(result)).To(Equal(1))
		})

		It("ignores junk before the first certicate", func() {
			certs := fmt.Sprintf("abcdefg %s\n%s\n", cert1, cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(result[1]).To(Equal(cert1))
			Expect(len(result)).To(Equal(2))
		})

		It("ignores junk after the last certicate", func() {
			certs := fmt.Sprintf("%s\n%s\n abcdefghij", cert1, cert1)

			result := cert.SplitCerts(certs)
			Expect(result[0]).To(Equal(cert1))
			Expect(result[1]).To(Equal(cert1))
			Expect(len(result)).To(Equal(2))
		})

		It("returns an empty slice for an empty string", func() {
			result := cert.SplitCerts("")
			Expect(len(result)).To(Equal(0))
		})

		It("returns an empty slice for an non-empty string that does not contain any certificates", func() {
			result := cert.SplitCerts("abcdefghij")
			Expect(len(result)).To(Equal(0))
		})
	})

	Describe("DeleteFile()", func() {
		var (
			fakeFs *fakesys.FakeFileSystem
		)

		BeforeEach(func() {
			fakeFs = fakesys.NewFakeFileSystem()
		})

		It("only deletes the files with the given prefix", func() {
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_1.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_2.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/different_file_1.bar", "goodbye")
			fakeFs.SetGlob("/path/to/delete/stuff/in/delete_me_*", []string{
				"/path/to/delete/stuff/in/delete_me_1.foo",
				"/path/to/delete/stuff/in/delete_me_2.foo",
			})
			count, err := cert.DeleteFiles(fakeFs, "/path/to/delete/stuff/in/", "delete_me_")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(2))
			Expect(countFiles(fakeFs, "/path/to/delete/stuff/in/")).To(Equal(1))
		})

		It("only deletes the files in the given path", func() {
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_1.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_2.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/other/things/in/delete_me_3.foo", "goodbye")
			fakeFs.SetGlob("/path/to/delete/stuff/in/delete_me_*", []string{
				"/path/to/delete/stuff/in/delete_me_1.foo",
				"/path/to/delete/stuff/in/delete_me_2.foo",
			})
			count, err := cert.DeleteFiles(fakeFs, "/path/to/delete/stuff/in/", "delete_me_")
			Expect(err).ToNot(HaveOccurred())
			Expect(count).To(Equal(2))
			Expect(countFiles(fakeFs, "/path/to/delete/stuff/in/")).To(Equal(0))
			Expect(countFiles(fakeFs, "/path/to/other/things/in/")).To(Equal(1))
		})

		It("returns an error when glob fails", func() {
			fakeFs.GlobErr = errors.New("couldn't walk")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_1.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_2.bar", "goodbye")
			count, err := cert.DeleteFiles(fakeFs, "/path/to/delete/stuff/in/", "delete_me_")
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(0))
		})

		It("returns an error when RemoveAll() fails", func() {
			fakeFs.RemoveAllError = errors.New("couldn't delete")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_1.foo", "goodbye")
			fakeFs.WriteFileString("/path/to/delete/stuff/in/delete_me_2.bar", "goodbye")
			fakeFs.SetGlob("/path/to/delete/stuff/in/delete_me_*", []string{
				"/path/to/delete/stuff/in/delete_me_1.foo",
				"/path/to/delete/stuff/in/delete_me_2.bar",
			})
			count, err := cert.DeleteFiles(fakeFs, "/path/to/delete/stuff/in/", "delete_me_")
			Expect(err).To(HaveOccurred())
			Expect(count).To(Equal(0))
		})
	})

	Describe("UbuntuCertManager", func() {
		var (
			fakeFs  *fakesys.FakeFileSystem
			fakeCmd *fakesys.FakeCmdRunner
		)

		BeforeEach(func() {
			fakeFs = fakesys.NewFakeFileSystem()
			fakeCmd = fakesys.NewFakeCmdRunner()
			fakeCmd.AddCmdResult("/usr/sbin/update-ca-certificates", fakesys.FakeCmdResult{
				Stdout:     "",
				Stderr:     "",
				ExitStatus: 0,
				Sticky:     true,
			})
		})

		It("writes 1 cert to a file", func() {
			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)
			err := ubuntuCertManager.UpdateCertificates(cert1)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt")).To(BeTrue())

		})

		It("writes each cert to its own file", func() {
			certs := fmt.Sprintf("%s\n%s\n", cert1, cert1)

			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)
			err := ubuntuCertManager.UpdateCertificates(certs)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt")).To(BeTrue())
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-2.crt")).To(BeTrue())
			Expect(countFiles(fakeFs, "/usr/local/share/ca-certificates")).To(Equal(2))
		})

		It("deletes exisitng cert files before writing new ones", func() {
			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)

			certs := fmt.Sprintf("%s\n%s\n", cert1, cert1)
			err := ubuntuCertManager.UpdateCertificates(certs)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt")).To(BeTrue())
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-2.crt")).To(BeTrue())
			Expect(countFiles(fakeFs, "/usr/local/share/ca-certificates")).To(Equal(2))

			fakeFs.SetGlob("/usr/local/share/ca-certificates/bosh-trusted-cert-*", []string{
				"/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt",
				"/usr/local/share/ca-certificates/bosh-trusted-cert-2.crt",
			})
			ubuntuCertManager.UpdateCertificates(cert1)
			Expect(fakeFs.FileExists("/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt")).To(BeTrue())
			Expect(countFiles(fakeFs, "/usr/local/share/ca-certificates")).To(Equal(1))
		})

		It("returns an error when deleting old certs fails", func() {
			fakeFs.RemoveAllError = errors.New("NOT ALLOW")
			fakeFs.WriteFileString("/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt", "goodbye")
			fakeFs.SetGlob("/usr/local/share/ca-certificates/bosh-trusted-cert-*", []string{
				"/usr/local/share/ca-certificates/bosh-trusted-cert-1.crt",
			})

			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)

			err := ubuntuCertManager.UpdateCertificates("")
			Expect(err).To(HaveOccurred())
		})

		It("returns an error when writing new cert files fails", func() {
			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)

			fakeFs.WriteFileError = errors.New("NOT ALLOW")
			err := ubuntuCertManager.UpdateCertificates(cert1)
			Expect(err).To(HaveOccurred())
		})

		It("executes update cert command", func() {
			fakeCmd = fakesys.NewFakeCmdRunner()
			fakeCmd.AddCmdResult("/usr/sbin/update-ca-certificates", fakesys.FakeCmdResult{
				Stdout:     "",
				Stderr:     "",
				ExitStatus: 2,
				Error:      errors.New("command failed"),
			})

			ubuntuCertManager := cert.NewUbuntuCertManager(fakeFs, fakeCmd, log)
			err := ubuntuCertManager.UpdateCertificates(cert1)
			Expect(err).To(HaveOccurred())
		})
	})
})

func countFiles(fs system.FileSystem, dir string) (count int) {
	fs.Walk(dir, func(path string, info os.FileInfo, err error) error {
		count++
		return nil
	})
	return
}
