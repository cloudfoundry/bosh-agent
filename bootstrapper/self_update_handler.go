package bootstrapper

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os/exec"
	"strings"

	"github.com/cloudfoundry/bosh-agent/logger"
)

type SelfUpdateHandler struct {
	Logger logger.Logger
}

func (h *SelfUpdateHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	defer func() { // catch panics
		panicValue := recover()
		if panicValue != nil {
			rw.WriteHeader(http.StatusInternalServerError)
			if err, ok := panicValue.(error); ok {
				h.Logger.Error("SelfUpdateHandler", "failed: %s", err.Error())
			} else {
				h.Logger.Error("SelfUpdateHandler", "failed but no idea why, sorry!", panicValue)
			}
		}
	}()

	if req.Method != "PUT" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	tmpDir, err := ioutil.TempDir("", "work-tmp")
	panicIfError(err)

	tarCommand := exec.Command("tar", "xvfz", "-")
	tarCommand.Dir = tmpDir

	stdInPipe, err := tarCommand.StdinPipe()
	panicIfError(err)

	panicIfError(tarCommand.Start())

	_, err = io.Copy(stdInPipe, req.Body)
	panicIfError(err)
	panicIfError(req.Body.Close())
	panicIfError(stdInPipe.Close())

	exitStatus := getExitStatus(tarCommand.Wait())
	if exitStatus != 0 {
		rw.WriteHeader(http.StatusBadRequest)
		h.Logger.Error("SelfUpdateHandler", "`%s` exited with %d", strings.Join(tarCommand.Args, " "), exitStatus)
		return
	}

	installShCommand := exec.Command(fmt.Sprintf("./%s", InstallScriptName))
	installShCommand.Dir = tmpDir
	panicIfError(installShCommand.Start())
	exitStatus = getExitStatus(installShCommand.Wait())
	if exitStatus != 0 {
		rw.WriteHeader(StatusUnprocessableEntity)
		h.Logger.Error("SelfUpdateHandler", "`%s` exited with %d", strings.Join(installShCommand.Args, " "), exitStatus)
		return
	}

	rw.Write(([]byte)(fmt.Sprintf("Your tarball was installed to %s", tmpDir)))
}
