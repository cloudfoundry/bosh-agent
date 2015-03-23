package bootstrapper

import (
	"net/http"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/package_installer"
	"github.com/cloudfoundry/bosh-agent/logger"
)

type SelfUpdateHandler struct {
	Logger           logger.Logger
	packageInstaller package_installer.PackageInstaller
}

func (h *SelfUpdateHandler) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method != "PUT" {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var err package_installer.PackageInstallerError

	err = h.packageInstaller.Install(req.Body)
	if err != nil {
		if err.SystemError() {
			rw.WriteHeader(http.StatusInternalServerError)
		} else {
			rw.WriteHeader(StatusUnprocessableEntity)
		}
		rw.Write([]byte(err.Error()))
		h.Logger.Error("SelfUpdateHandler", err.Error())
		return
	}

	rw.Write(([]byte)("Your tarball was installed."))
}

func (h *SelfUpdateHandler) internalServerError(err error, rw http.ResponseWriter) {
	rw.WriteHeader(http.StatusInternalServerError)
	h.Logger.Error("SelfUpdateHandler", "failed: %s", err.Error())
}
