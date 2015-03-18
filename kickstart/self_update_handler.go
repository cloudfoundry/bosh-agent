package kickstart

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/cloudfoundry/bosh-agent/errors"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type SelfUpdateHandler struct {
	AllowedDNs *DNPatterns

	Logger logger.Logger
}

func (h *SelfUpdateHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "PUT" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	err := h.AllowedDNs.Verify(req)
	if err != nil {
		rw.WriteHeader(http.StatusUnauthorized)
		h.Logger.Error("SelfUpdateHandler", errors.WrapError(err, "Unauthorized access").Error())
		return
	}

	tmpDir, err := ioutil.TempDir("", "test-tmp")
	if err != nil {
		panic(err)
	}

	tarCommand := exec.Command("tar", "xvfz", "-")
	tarCommand.Dir = tmpDir

	stdInPipe, err := tarCommand.StdinPipe()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "tar failed: %s", err.Error())
		return
	}

	err = tarCommand.Start()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "tar failed: %s", err.Error())
		return
	}

	_, err = io.Copy(stdInPipe, req.Body)
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "tar failed: %s", err.Error())
		return
	}

	req.Body.Close()
	stdInPipe.Close()

	exitStatus := getExitStatus(tarCommand.Wait())

	if exitStatus != 0 {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "`%s` exited with %d", strings.Join(tarCommand.Args, " "), exitStatus)
		return
	}

	//	_, err := tarCommand.CombinedOutput()
	//	if err != nil {
	//		rw.WriteHeader(http.StatusBadRequest)
	//		h.Logger.Error("SelfUpdateHandler", "tar failed: %s", err.Error())
	//		return
	//	}

	execCommand := exec.Command(fmt.Sprintf("./%s", InstallScriptName))
	execCommand.Dir = tmpDir
	err = execCommand.Start()
	if err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "`%s` exited with %d", strings.Join(tarCommand.Args, " "), exitStatus)
		return
	}

	err = execCommand.Wait()
	if err != nil {
		fmt.Println(err)
		return
	}

	rw.Write(([]byte)(fmt.Sprintf("Your tarball was installed to %s", tmpDir)))
}
