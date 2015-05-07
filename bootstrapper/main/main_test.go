package main_test

import (
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/spec"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"
)

var bin string

var _ = SynchronizedBeforeSuite(func() []byte {
	bootstrapBin, err := gexec.Build("github.com/cloudfoundry/bosh-agent/bootstrapper/main")
	Expect(err).ToNot(HaveOccurred())
	return []byte(bootstrapBin)
}, func(payload []byte) {
	bin = string(payload)
})

var _ = SynchronizedAfterSuite(func() {}, func() {
	gexec.CleanupBuildArtifacts()
})

var _ = Describe("Main", func() {
	var session *gexec.Session

	Describe("download", func() {
		var listener net.Listener

		BeforeEach(func() {
			installScript := "#!/bin/bash\necho hello from install script \n"
			tarballPath := spec.CreateTarball(installScript)
			listener = spec.StartDownloadServer(9003, tarballPath, spec.CertFor("director"))
		})

		AfterEach(func() {
			if listener != nil {
				listener.Close()
			}
		})

		It("downloads and runs the installer", func() {
			path := "/tarball.tgz"
			url := "https://localhost:9003" + path
			cmd := exec.Command(
				bin,
				"download",
				url,
				"-certFile", spec.FixtureFilename("certs/bootstrapper.crt"),
				"-keyFile", spec.FixtureFilename("certs/bootstrapper.key"),
				"-caPemFile", spec.FixtureFilename("certs/rootCA.pem"),
				"-allowedName", "*",
			)
			var startErr error
			session, startErr = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(startErr).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("hello from install script"))
			Expect(session.Out).To(gbytes.Say("Download succeeded"))
		})
	})

	Describe("listen", func() {
		var session *gexec.Session
		var port = 4443 + GinkgoParallelNode()
		var url = fmt.Sprintf("https://localhost:%d/self-update", port)

		BeforeEach(func() {
			cmd := exec.Command(
				bin,
				"listen",
				strconv.Itoa(port),
				"-certFile", spec.FixtureFilename("certs/bootstrapper.crt"),
				"-keyFile", spec.FixtureFilename("certs/bootstrapper.key"),
				"-caPemFile", spec.FixtureFilename("certs/rootCA.pem"),
				"-allowedName", "*",
			)
			var err error
			session, err = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			session.Kill()
			Eventually(session).Should(gexec.Exit())
		})

		It("accepts PUT requests and runs the installer", func() {
			installScript := "#!/bin/bash\necho hello from install script \n"
			tarballPath := spec.CreateTarball(installScript)
			resp, err := spec.HttpPut(url, tarballPath, spec.CertFor("director"))
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))

			session.Kill()
			outContents := session.Wait().Out.Contents()
			Expect(outContents).To(ContainSubstring("hello from install script"))
			Expect(outContents).To(ContainSubstring("successfully installed package"))
		})
	})
})
