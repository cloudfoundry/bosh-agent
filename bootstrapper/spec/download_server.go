package spec

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path"

	. "github.com/onsi/gomega"
)

func StartDownloadServer(port int, tarballPath string, directorCert *tls.Certificate) net.Listener {
	ln, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	Expect(err).ToNot(HaveOccurred())
	tlsListener := tls.NewListener(ln, &tls.Config{
		Certificates: []tls.Certificate{*directorCert},
		ClientCAs:    CertPool(),
	})

	server := &http.Server{}
	server.Handler = http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if req.Method != "GET" {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if req.URL.Path != "/tarball.tgz" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/binary")
		tarReader, err := os.Open(tarballPath)
		Expect(err).ToNot(HaveOccurred())
		_, err = io.Copy(w, tarReader)
		Expect(err).ToNot(HaveOccurred())
		tarReader.Close()
	})
	go server.Serve(tlsListener)
	return tlsListener
}

func CreateTarball(installScript string) string {
	tmpDir, err := ioutil.TempDir("", "test-tmp")
	Expect(err).ToNot(HaveOccurred())
	ioutil.WriteFile(path.Join(tmpDir, "install.sh"), ([]byte)(installScript), 0755)
	tarCmd := exec.Command("tar", "cfz", "tarball.tgz", "install.sh")
	tarCmd.Dir = tmpDir
	_, err = tarCmd.CombinedOutput()
	Expect(err).ToNot(HaveOccurred())

	return path.Join(tmpDir, "tarball.tgz")
}

func GetFreePort() int {
	listener, err := net.ListenTCP("tcp", nil)
	Expect(err).ToNot(HaveOccurred())

	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()
	return port
}
