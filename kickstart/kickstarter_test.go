package kickstart_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	fmt "fmt"
	"io/ioutil"
	"log"
	"net"
	http "net/http"
	os "os"
	"os/exec"
	"path"

	"bytes"
	. "github.com/cloudfoundry/bosh-agent/kickstart"
)

var _ = Describe("kickstart", mainDesc)

func mainDesc() {
	var (
		tmpDir string
		k      *Kickstart
		port   int
	)

	directorCert := certFor("director")

	BeforeEach(func() {
		var err error
		tmpDir, err = ioutil.TempDir("", "test-tmp")
		Expect(err).ToNot(HaveOccurred())

		k = &Kickstart{
			CertFile:  fixtureFilename("certs/kickstart.crt"),
			KeyFile:   fixtureFilename("certs/kickstart.key"),
			CACertPem: (string)(fixtureData("certs/rootCA.pem")),
			Logger:    log.New(&bytes.Buffer{}, "", 0),
		}

		installScript := fmt.Sprintf("#!/bin/bash\necho hiya > %s/install.log\n", tmpDir)
		ioutil.WriteFile(path.Join(tmpDir, INSTALL_SCRIPT_NAME), ([]byte)(installScript), 0755)
		tarCmd := exec.Command("tar", "cfz", "tarball.tgz", INSTALL_SCRIPT_NAME)
		tarCmd.Dir = tmpDir
		_, err = tarCmd.CombinedOutput()
		Expect(err).ToNot(HaveOccurred())

		port = getFreePort()
	})

	AfterEach(func() {
		os.RemoveAll(tmpDir)
	})

	Describe("#Listen", func() {
		It("returns an error when the port is already taken", func() {
			_, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
			Expect(err).ToNot(HaveOccurred())
			err = k.Listen(port)
			Expect(err.Error()).To(ContainSubstring("address already in use"))
		})

		It("listens on a given port", func() {
			err := k.Listen(port)
			Expect(err).ToNot(HaveOccurred())
			url := fmt.Sprintf("https://localhost:%d/self-update", port)
			resp, err := httpPut(url, path.Join(tmpDir, "tarball.tgz"), directorCert)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.StatusCode).To(Equal(http.StatusOK))
		})

		It("identifies itself with the provided key", func() {
			err := k.Listen(port)
			Expect(err).ToNot(HaveOccurred())
			url := fmt.Sprintf("https://localhost:%d/self-update", port)
			resp, err := httpPut(url, path.Join(tmpDir, "tarball.tgz"), directorCert)
			Expect(err).ToNot(HaveOccurred())
			Expect(resp.TLS.PeerCertificates[0].Subject.Organization[0]).To(Equal("bosh.kickstart"))
		})
	})

	Describe("PUT /self-update", func() {
		BeforeEach(func() { k.Listen(port) })

		It("expands uploaded tarball and runs install.sh", func() {
			url := fmt.Sprintf("https://localhost:%d/self-update", port)

			_, err := httpPut(url, path.Join(tmpDir, "tarball.tgz"), directorCert)
			Expect(err).ToNot(HaveOccurred())
			installLog, err := ioutil.ReadFile(path.Join(tmpDir, "install.log"))
			Expect((string)(installLog)).To(Equal("hiya\n"))
		})

		It("rejects requests without a client certificate", func() {
			url := fmt.Sprintf("https://localhost:%d/self-update", port)
			_, err := httpPut(url, path.Join(tmpDir, "tarball.tgz"), nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bad certificate"))
		})

		It("rejects requests when the client certificate isn't signed by the given CA", func() {
			url := fmt.Sprintf("https://localhost:%d/self-update", port)
			_, err := httpPut(url, path.Join(tmpDir, "tarball.tgz"), certFor("directorWithWrongCA"))
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("bad certificate"))
		})
	})
}
