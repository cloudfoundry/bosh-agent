package downloader_test

import (
	"crypto/tls"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/downloader"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/installer"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/spec"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
	boshlogger "github.com/cloudfoundry/bosh-agent/logger"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

var _ = Describe("Downloader", func() {
	var (
		dl           *downloader.Downloader
		tarballURL   string
		listener     net.Listener
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
		logWriter = spec.NewCapturableWriter(GinkgoWriter)
		sysLog := log.New(logWriter, "", 0)
		logger = boshlogger.New(boshlogger.LevelDebug, sysLog, sysLog)
		directorCert = spec.CertFor("director")
		port = spec.GetFreePort()
		tmpDir, err = ioutil.TempDir("", "test-tmp")
		Expect(err).ToNot(HaveOccurred())

		installScript := fmt.Sprintf("#!/bin/bash\necho hiya > %s/install.log\n", tmpDir)
		tarballPath = spec.CreateTarball(installScript)

		allowedNames = []string{"*"}
		system := system.NewOsSystem()
		i = installer.New(system)
	})

	JustBeforeEach(func() {
		listener = spec.StartDownloadServer(port, tarballPath, directorCert)
		tarballURL = fmt.Sprintf("https://localhost:%d/tarball.tgz", port)

		config, err := auth.NewSSLConfig(
			spec.FixtureFilename("certs/bootstrapper.crt"),
			spec.FixtureFilename("certs/bootstrapper.key"),
			(string)(spec.FixtureData("certs/rootCA.pem")),
			allowedNames,
		)
		Expect(err).ToNot(HaveOccurred())
		dl = downloader.NewDownloader(config, i)
	})

	AfterEach(func() {
		if listener != nil {
			listener.Close()
		}
	})

	It("GETs the given URL, opens the tarball, and runs install.sh", func() {
		err := dl.Download(logger, tarballURL)
		Expect(err).ToNot(HaveOccurred())
		installLog, err := ioutil.ReadFile(path.Join(tmpDir, "install.log"))
		Expect(err).ToNot(HaveOccurred())
		Expect((string)(installLog)).To(Equal("hiya\n"))
	})

	Context("when the download url is bad", func() {
		It("returns an http error", func() {
			err := dl.Download(logger, fmt.Sprintf("https://localhost:%d/foobar", port))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("Not Found"))
		})
	})

	Context("when the downloaded file is bad", func() {
		BeforeEach(func() {
			tarballPath = spec.CreateTarball("foooooooooooooooooooo")
		})
		It("returns a file error", func() {
			err := dl.Download(logger, tarballURL)
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
			err := dl.Download(logger, tarballURL)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("didn't match allowed distinguished names"))
			_, err = os.Stat(path.Join(tmpDir, "install.log"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("no such file"))
		})
	})
})
