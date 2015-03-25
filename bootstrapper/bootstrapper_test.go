package bootstrapper_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/package_installer"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/spec"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"

	. "github.com/cloudfoundry/bosh-agent/bootstrapper"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("bootstrapper", mainDesc)

func mainDesc() {
	var (
		err          error
		bootstrapper *Bootstrapper
		tmpDir       string
		tarballPath  string

		logWriter        spec.CapturableWriter
		allowedNames     []string
		port             int
		directorCert     *tls.Certificate
		packageInstaller package_installer.PackageInstaller
	)

	BeforeEach(func() {
		logWriter = spec.NewCapturableWriter(GinkgoWriter)
		directorCert = spec.CertFor("director")
		port = spec.GetFreePort()
		tmpDir, err = ioutil.TempDir("", "test-tmp")
		Expect(err).ToNot(HaveOccurred())

		installScript := fmt.Sprintf("#!/bin/bash\necho hiya > %s/install.log\n", tmpDir)
		tarballPath = spec.CreateTarball(installScript)

		allowedNames = []string{"*"}
		system := system.NewOsSystem()
		packageInstaller = package_installer.New(system)
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
		bootstrapper.StopListening()
		bootstrapper.WaitForServerToExit()
	})

	// remember to clean up after ourselves when install.sh finishes?

	Describe("#Download", func() {
		var (
			tarballURL string
			listener   net.Listener
		)

		JustBeforeEach(func() {
			listener = spec.StartDownloadServer(port, tarballPath, directorCert)
			tarballURL = fmt.Sprintf("https://localhost:%d/tarball.tgz", port)

			bootstrapper = &Bootstrapper{
				CertFile:         spec.FixtureFilename("certs/bootstrapper.crt"),
				KeyFile:          spec.FixtureFilename("certs/bootstrapper.key"),
				CACertPem:        (string)(spec.FixtureData("certs/rootCA.pem")),
				AllowedNames:     allowedNames,
				Logger:           log.New(logWriter, "", 0),
				PackageInstaller: packageInstaller,
			}
		})

		AfterEach(func() {
			if listener != nil {
				listener.Close()
			}
		})

		It("GETs the given URL, opens the tarball, and runs install.sh", func() {
			err := bootstrapper.Download(tarballURL)
			Expect(err).ToNot(HaveOccurred())
			installLog, err := ioutil.ReadFile(path.Join(tmpDir, "install.log"))
			Expect(err).ToNot(HaveOccurred())
			Expect((string)(installLog)).To(Equal("hiya\n"))
		})

		Context("when the download url is bad", func() {
			It("returns an http error", func() {
				err := bootstrapper.Download(fmt.Sprintf("https://localhost:%d/foobar", port))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("Not Found"))
			})
		})

		Context("when the downloaded file is bad", func() {
			BeforeEach(func() {
				tarballPath = spec.CreateTarball("foooooooooooooooooooo")
			})
			It("returns a file error", func() {
				err := bootstrapper.Download(tarballURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("install.sh"))
			})
		})

		Context("when server cert doesn't match client cert rules", func() {
			BeforeEach(func() {
				allowedNames = []string{"o=not.bosh.director"}
			})

			It("rejects the request", func() {
				logWriter.Capture("Fake Bosh Server")
				err := bootstrapper.Download(tarballURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("didn't match allowed distinguished names"))
				_, err = os.Stat(path.Join(tmpDir, "install.log"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("no such file"))
			})
		})

		Context("with a malformed AllowedNames list", func() {
			BeforeEach(func() { allowedNames = []string{"invalid=value"} })
			It("returns an error", func() {
				err := bootstrapper.Download(tarballURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Invalid AllowedNames: Unknown field 'invalid'"))
			})
		})

		Context("with an empty AllowedNames list", func() {
			BeforeEach(func() { allowedNames = []string{} })
			It("returns an error", func() {
				err := bootstrapper.Download(tarballURL)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("AllowedNames must be specified"))
			})
		})
	})

	Describe("#Listen", func() {
		JustBeforeEach(func() {
			bootstrapper = &Bootstrapper{
				CertFile:         spec.FixtureFilename("certs/bootstrapper.crt"),
				KeyFile:          spec.FixtureFilename("certs/bootstrapper.key"),
				CACertPem:        (string)(spec.FixtureData("certs/rootCA.pem")),
				AllowedNames:     allowedNames,
				Logger:           log.New(logWriter, "", 0),
				PackageInstaller: packageInstaller,
			}
		})

		It("returns an error when the port is already taken", func() {
			port := spec.GetFreePort()
			_, err = net.ListenTCP("tcp", &net.TCPAddr{Port: port})
			Expect(err).ToNot(HaveOccurred())
			err = bootstrapper.Listen(port)
			Expect(err.Error()).To(ContainSubstring("address already in use"))
		})

		It("listens on a given port", func() {
			err = bootstrapper.Listen(port)
			Expect(err).ToNot(HaveOccurred())
			url := fmt.Sprintf("https://localhost:%d/self-update", port)
			resp, err := spec.HttpPut(url, tarballPath, directorCert)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		Context("with a malformed AllowedNames list", func() {
			BeforeEach(func() { allowedNames = []string{"invalid=value"} })
			It("returns an error", func() {
				err = bootstrapper.Listen(port)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("Invalid AllowedNames: Unknown field 'invalid'"))
			})
		})

		Context("with an empty AllowedNames list", func() {
			BeforeEach(func() { allowedNames = []string{} })
			It("returns an error", func() {
				err = bootstrapper.Listen(port)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(Equal("AllowedNames must be specified"))
			})
		})

		Describe("TLS handshaking", func() {
			var url string

			BeforeEach(func() {
				url = fmt.Sprintf("https://localhost:%d/self-update", port)
			})

			JustBeforeEach(func() {
				bootstrapper.Listen(port)
			})

			It("identifies itself with the provided key", func() {
				Expect(err).ToNot(HaveOccurred())
				resp, err := spec.HttpPut(url, tarballPath, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.TLS.PeerCertificates[0].Subject.Organization[0]).To(Equal("bosh.bootstrapper"))
			})

			It("rejects requests without a client certificate", func() {
				logWriter.Ignore("client didn't provide a certificate")
				_, err = spec.HttpPut(url, tarballPath, nil)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bad certificate"))
				Expect(spec.FileExists(path.Join(tmpDir, "install.log"))).To(BeFalse())
			})

			It("rejects requests when the client certificate isn't signed by the given CA", func() {
				logWriter.Ignore("client didn't provide a certificate")
				_, err = spec.HttpPut(url, tarballPath, spec.CertFor("directorWithWrongCA"))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("bad certificate"))
				Expect(spec.FileExists(path.Join(tmpDir, "install.log"))).To(BeFalse())
			})

			Context("when the client cert's distinguished name is not permitted", func() {
				BeforeEach(func() { allowedNames = []string{"o=bosh.not-director"} })
				It("rejects the request", func() {
					logWriter.Capture("Unauthorized")
					resp, err := spec.HttpPut(url, tarballPath, directorCert)
					Expect(err).ToNot(HaveOccurred())
					Expect(resp.StatusCode).To(Equal(http.StatusUnauthorized))
					Expect(spec.FileExists(path.Join(tmpDir, "install.log"))).To(BeFalse())
					Expect(logWriter.Captured()).To(ContainSubstring("ERROR - Unauthorized access: Subject"))
				})
			})

		})

		Describe("PUT /self-update", func() {
			var url string

			BeforeEach(func() {
				url = fmt.Sprintf("https://localhost:%d/self-update", port)
			})

			JustBeforeEach(func() {
				bootstrapper.Listen(port)
			})

			It("expands uploaded tarball and runs install.sh", func() {
				resp, err := spec.HttpPut(url, tarballPath, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(resp.StatusCode).To(Equal(http.StatusOK))

				installLog, err := ioutil.ReadFile(path.Join(tmpDir, "install.log"))
				Expect(err).ToNot(HaveOccurred())
				Expect((string)(installLog)).To(Equal("hiya\n"))
			})

			It("returns an UnproccessableEntity when there are problems with the payload", func() {
				logWriter.Capture("SelfUpdateHandler")

				req, err := http.NewRequest("PUT", url, strings.NewReader("busted tar"))
				Expect(err).ToNot(HaveOccurred())
				resp, err := spec.HttpClient(directorCert).Do(req)
				Expect(err).ToNot(HaveOccurred())

				Expect(resp.StatusCode).To(Equal(StatusUnprocessableEntity))

				contents, err := ioutil.ReadAll(resp.Body)
				Expect(err).ToNot(HaveOccurred())
				expectedError := "`tar xvfz -` exited with"
				Expect(string(contents)).To(ContainSubstring(expectedError))
				Expect(logWriter.Captured()).To(ContainSubstring(expectedError))
			})

			Context("when the system has errors", func() {
				BeforeEach(func() {
					packageInstaller = erroringPackageInstaller{message: "Ahhhhhhh!!!"}
				})

				It("returns an InternalServerError when appropriate", func() {
					logWriter.Capture("SelfUpdateHandler")
					resp, err := spec.HttpPut(url, tarballPath, directorCert)
					Expect(err).ToNot(HaveOccurred())

					Expect(resp.StatusCode).To(Equal(http.StatusInternalServerError))
					contents, err := ioutil.ReadAll(resp.Body)
					Expect(err).ToNot(HaveOccurred())

					expectedError := "Ahhhhhhh!!!"
					Expect(string(contents)).To(ContainSubstring(expectedError))
					Expect(logWriter.Captured()).To(ContainSubstring(expectedError))
				})
			})
		})

		Describe("for other endpoints", func() {
			JustBeforeEach(func() { Expect(bootstrapper.Listen(port)).ToNot(HaveOccurred()) })

			It("returns 404 for GET /self-update", func() {
				url := fmt.Sprintf("https://localhost:%d/self-update", port)
				response, err := spec.HttpDo("GET", url, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusMethodNotAllowed))
			})

			It("returns 404 for POST /self-update", func() {
				url := fmt.Sprintf("https://localhost:%d/self-update", port)
				response, err := spec.HttpDo("POST", url, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusMethodNotAllowed))
			})

			It("returns 404 for DELETE /self-update", func() {
				url := fmt.Sprintf("https://localhost:%d/self-update", port)
				response, err := spec.HttpDo("DELETE", url, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusMethodNotAllowed))
			})

			It("returns 404 for GET /foo", func() {
				url := fmt.Sprintf("https://localhost:%d/foo", port)
				response, err := spec.HttpDo("GET", url, directorCert)
				Expect(err).ToNot(HaveOccurred())
				Expect(response.StatusCode).To(Equal(http.StatusNotFound))
			})
		})
	})
}

type erroringPackageInstaller struct {
	message string
}

type erroringPackageInstallerError struct {
	message string
}

func (erroringPackageInstallerError erroringPackageInstallerError) Error() string {
	return erroringPackageInstallerError.message
}

func (erroringPackageInstallerError erroringPackageInstallerError) SystemError() bool {
	return true
}

func (erroringPackageInstaller erroringPackageInstaller) Install(reader io.Reader) package_installer.PackageInstallerError {
	return erroringPackageInstallerError{message: erroringPackageInstaller.message}
}
