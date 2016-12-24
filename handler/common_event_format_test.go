package handler_test

import (
	"github.com/cloudfoundry/bosh-agent/handler"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"net/http/httptest"
)

var _ = Describe("CommonEventFormat", func() {
	It("should produce CEF string", func() {
		cef := handler.NewCommonEventFormat()
		request := httptest.NewRequest("GET", "https://user:pass@127.0.0.1:6900/blobs", nil)
		request.SetBasicAuth("username", "password")
		request.Header.Set("HTTP_HOST", "host.example.com")
		request.Header.Set("HTTP_X_REAL_IP", "12.12.34.56")
		request.Header.Set("HTTP_X_FORWARDED_FOR", "forward")
		request.Header.Set("HTTP_X_FORWARDED_PROTO", "proto")
		request.Header.Set("HTTP_USER_AGENT", "my.agent")
		cefLog, err := cef.ProduceEventLog(request, 400, `{"reason": "no reason"}`)

		Expect(err).NotTo(HaveOccurred())
		Expect(cefLog).To(ContainSubstring("CEF:0|CloudFoundry|BOSH|1|agent_api|/blobs|7|duser=username requestMethod=GET"))
		Expect(cefLog).To(ContainSubstring("src="))
		Expect(cefLog).To(ContainSubstring("spt="))
		Expect(cefLog).To(ContainSubstring("shost"))
		Expect(cefLog).To(ContainSubstring(`cs1=HOST=host.example.com&X_REAL_IP=12.12.34.56&X_FORWARDED_FOR=forward&X_FORWARDED_PROTO=proto&USER_AGENT=my.agent cs1Label=httpHeaders`))
		Expect(cefLog).To(ContainSubstring(`cs2=basic cs2Label=authType cs3=400 cs3Label=responseStatus cs4={"reason": "no reason"} cs4Label=statusReason`))

	})
})
