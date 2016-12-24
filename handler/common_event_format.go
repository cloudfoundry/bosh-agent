package handler

import (
	"bytes"
	"fmt"
	"net/http"
	"os"
	"strings"
)

type CommonEventFormat interface {
	ProduceEventLog(*http.Request, int, string) string
}

func NewCommonEventFormat() CommonEventFormat {
	return concreteCommonEventFormat{}
}

type concreteCommonEventFormat struct{}

func (cef concreteCommonEventFormat) ProduceEventLog(request *http.Request, respStatusCode int, respJSON string) string {
	const cefVersion = 0
	const deviceVendor = "CloudFoundry"
	const deviceProduct = "BOSH"
	const deviceVersion = "1"
	const signatureID = "agent_api"

	name := request.URL.Path
	severity := 1
	if respStatusCode >= 400 {
		severity = 7
	}
	extension := ""

	username, _, _ := request.BasicAuth()

	hostname, _ := os.Hostname()
	extension = fmt.Sprintf(
		`duser=%s requestMethod=%s src=%s spt=%s shost=%s cs1=%s cs1Label=httpHeaders cs2=basic cs2Label=authType cs3=%v cs3Label=responseStatus`,
		username, request.Method, strings.Split(request.RemoteAddr, ":")[0], strings.Split(request.RemoteAddr, ":")[1], hostname, "", respStatusCode)
	if respStatusCode >= 400 {
		var buffer bytes.Buffer

		buffer.WriteString(extension)
		buffer.WriteString(fmt.Sprintf("cs4=%s cs4Label=statusReason", respJSON))
		extension = buffer.String()
	}

	return fmt.Sprintf("CEF:%v|%s|%s|%s|%s|%s|%v|%s", cefVersion, deviceVendor, deviceProduct, deviceVersion, signatureID, name, severity, extension)
}
