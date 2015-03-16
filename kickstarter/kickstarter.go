package kickstarter

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"sync"
)

type Kickstarter struct {
	CertFile string
	KeyFile  string
	wg       sync.WaitGroup
}

const INSTALL_SCRIPT_NAME = "install.sh"

func (k *Kickstarter) Listen(port int) error {
	serveMux := http.NewServeMux()
	serveMux.HandleFunc("/", rootHander)

	server := &http.Server{Handler: serveMux}

	config := &tls.Config{
		NextProtos: []string{"http/1.1"},
	}

	listener, err := net.ListenTCP("tcp", &net.TCPAddr{Port: port})
	if err != nil {
		return err
	}

	config.Certificates = make([]tls.Certificate, 1)
	config.Certificates[0], _ = tls.LoadX509KeyPair(k.CertFile, k.KeyFile)
	tlsListener := tls.NewListener(listener, config)

	k.wg.Add(1)
	go k.run(server, tlsListener)
	return nil
}

func (k *Kickstarter) run(server *http.Server, tlsListener net.Listener) {
	defer k.wg.Done()
	err := server.Serve(tlsListener)
	if err != nil {
		fmt.Printf("run(): %s\n", err)
	}
}

func (k *Kickstarter) WaitForServerToExit() {
	k.wg.Wait()
}

func rootHander(rw http.ResponseWriter, req *http.Request) {
	tmpDir, err := ioutil.TempDir("", "test-tmp")
	tarCommand := exec.Command("tar", "xvfz", "-")
	tarCommand.Dir = tmpDir

	stdInPipe, err := tarCommand.StdinPipe()
	tarCommand.Start()
	_, err = io.Copy(stdInPipe, req.Body)
	if err != nil {
		fmt.Println(err)
	}
	req.Body.Close()
	stdInPipe.Close()
	err = tarCommand.Wait()
	if err != nil {
		fmt.Println(err)
	}

	execCommand := exec.Command(fmt.Sprintf("./%s", INSTALL_SCRIPT_NAME))
	execCommand.Dir = tmpDir
	err = execCommand.Start()
	if err != nil {
		fmt.Println(err)
	}

	err = execCommand.Wait()
	if err != nil {
		fmt.Println(err)
	}
}
