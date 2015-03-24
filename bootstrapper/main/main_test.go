package main_test

import (
	"net"
	"os/exec"

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
	var startErr error

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
				spec.FixtureFilename("certs/bootstrapper.crt"),
				spec.FixtureFilename("certs/bootstrapper.key"),
				spec.FixtureFilename("certs/rootCA.pem"),
				"*",
			)
			session, startErr = gexec.Start(cmd, GinkgoWriter, GinkgoWriter)
			Expect(startErr).ToNot(HaveOccurred())
			Eventually(session).Should(gexec.Exit(0))
			Expect(session.Out).To(gbytes.Say("hello from install script"))
			Expect(session.Out).To(gbytes.Say("Download succeeded"))
		})
	})

	XDescribe("listen", func() {})
})
