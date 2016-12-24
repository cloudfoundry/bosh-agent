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
		cefLog := cef.ProduceEventLog(request, 400, `{"reason": "no reason"}`)

		Expect(cefLog).To(Equal(`CEF:0|CloudFoundry|BOSH|1|agent_api|/blobs|7|duser=username requestMethod=GET src=192.0.2.1 spt=1234 shost=lake cs1= cs1Label=httpHeaders cs2=basic cs2Label=authType cs3=400 cs3Label=responseStatuscs4={"reason": "no reason"} cs4Label=statusReason`))
	})
})
