package listener_test

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path"
	"strings"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/installer"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/listener"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/spec"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
	boshlogger "github.com/cloudfoundry/bosh-agent/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Listener", func() {
	var (
		l            *listener.Listener
		logWriter    spec.CapturableWriter
		logger       boshlogger.Logger
		allowedNames []string
		port         int
		directorCert *tls.Certificate
		i            installer.Installer
		tmpDir       string
		tarballPath  string
	)

	BeforeEach(func() {
		var err error
		log.SetOutput(GinkgoWriter)
		logWriter = spec.NewCapturableWriter(GinkgoWriter)
		sysLog := log.New(logWriter, "", 0)
		logger = boshlogger.New(boshlogger.LevelDebug, sysLog, sysLog)
		directorCert = spec.CertFor("director")
		tmpDir, err = ioutil.TempDir("", "test-tmp")
		Expect(err).ToNot(HaveOccurred())
		installScript := fmt.Sprintf("#!/bin/bash\necho hiya > %s/install.log\n", tmpDir)
		tarballPath = spec.CreateTarball(installScript)
		allowedNames = []string{"*"}
		system := system.NewOsSystem()
		i = installer.New(system)
		port = spec.GetFreePort()
	})

	JustBeforeEach(func() {
		config, err := auth.NewSSLConfig(
			spec.FixtureFilename("certs/bootstrapper.crt"),
			spec.FixtureFilename("certs/bootstrapper.key"),
			(string)(spec.FixtureData("certs/rootCA.pem")),
			allowedNames,
		)
		Expect(err).ToNot(HaveOccurred())

		l = listener.NewListener(config, i)
	})

	AfterEach(func() {
		if l != nil {
			l.Close()
		}
	})

	Context("when the port is already taken", func() {
		var otherListener *net.TCPListener

		BeforeEach(func() {
			var err error
			otherListener, err = net.ListenTCP("tcp", &net.TCPAddr{Port: port})
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			otherListener.Close()
		})

		It("returns an error when the port is already taken", func() {
			err := l.ListenAndServe(logger, port)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("address already in use"))
		})
	})

	It("listens on a given port", func() {
		err := l.ListenAndServe(logger, port)
		Expect(err).ToNot(HaveOccurred())
		url := fmt.Sprintf("https://localhost:%d/self-update", port)
		resp, err := spec.HttpPut(url, tarballPath, directorCert)
		Expect(err).ToNot(HaveOccurred())
		Expect(resp.StatusCode).To(Equal(http.StatusOK))
	})

	Describe("TLS handshaking", func() {
		var url string

		BeforeEach(func() {
			url = fmt.Sprintf("https://localhost:%d/self-update", port)
		})

		JustBeforeEach(func() {
			err := l.ListenAndServe(logger, port)
			Expect(err).ToNot(HaveOccurred())
		})

		It("identifies itself with the provided key", func() {
			resp, err := spec.HttpPut(url, tarballPath, directorCert)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.TLS.PeerCertificates[0].Subject.Organization[0]).To(Equal("bosh.bootstrapper"))
		})

		It("rejects requests without a client certificate", func() {
			logWriter.Ignore("client didn't provide a certificate")
			_, err := spec.HttpPut(url, tarballPath, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bad certificate"))
			Expect(spec.FileExists(path.Join(tmpDir, "install.log"))).To(BeFalse())
		})

		It("rejects requests when the client certificate isn't signed by the given CA", func() {
			logWriter.Ignore("client didn't provide a certificate")
			_, err := spec.HttpPut(url, tarballPath, spec.CertFor("directorWithWrongCA"))
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
			l.ListenAndServe(logger, port)
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

			Expect(resp.StatusCode).To(Equal(listener.StatusUnprocessableEntity))

			contents, err := ioutil.ReadAll(resp.Body)
			Expect(err).ToNot(HaveOccurred())
			expectedError := "`tar xvfz -` exited with"
			Expect(string(contents)).To(ContainSubstring(expectedError))
			Expect(logWriter.Captured()).To(ContainSubstring(expectedError))
		})

		Context("when the system has errors", func() {
			BeforeEach(func() {
				i = erroringInstaller{message: "Ahhhhhhh!!!"}
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
		JustBeforeEach(func() { Expect(l.ListenAndServe(logger, port)).ToNot(HaveOccurred()) })

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

type erroringInstaller struct {
	message string
}

type erroringInstallerError struct {
	message string
}

func (erroringInstallerError erroringInstallerError) Error() string {
	return erroringInstallerError.message
}

func (erroringInstallerError erroringInstallerError) SystemError() bool {
	return true
}

func (erroringInstaller erroringInstaller) Install(reader io.Reader) installer.Error {
	return erroringInstallerError{message: erroringInstaller.message}
}
