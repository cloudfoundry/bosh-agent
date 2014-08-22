package client_test

import (
	"errors"
	"io/ioutil"
	"strings"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/davcli/client"
	davconf "github.com/cloudfoundry/bosh-agent/davcli/config"
	fakehttp "github.com/cloudfoundry/bosh-agent/http/fakes"
)

var _ = Describe("Client", func() {
	var (
		fakeHTTPClient *fakehttp.FakeClient
		config         davconf.Config
		client         Client
	)

	BeforeEach(func() {
		fakeHTTPClient = fakehttp.NewFakeClient()
		client = NewClient(config, fakeHTTPClient)
	})

	Describe("Get", func() {
		It("returns the response body from the given path", func() {
			fakeHTTPClient.SetMessage("response")

			responseBody, err := client.Get("/")
			Expect(err).NotTo(HaveOccurred())
			buf := make([]byte, 1024)
			n, _ := responseBody.Read(buf)
			Expect(string(buf[0:n])).To(Equal("response"))
		})

		Context("when the http request fails", func() {
			It("returns err", func() {
				fakeHTTPClient.Error = errors.New("")
				responseBody, err := client.Get("/")
				Expect(responseBody).To(BeNil())
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Put", func() {
		It("uploads the given content", func() {
			body := ioutil.NopCloser(strings.NewReader("content"))
			err := client.Put("/", body)
			Expect(err).NotTo(HaveOccurred())
			Expect(fakeHTTPClient.RequestBodies).To(Equal([]string{"content"}))
		})

		Context("when the http request fails", func() {
			It("returns err", func() {
				fakeHTTPClient.Error = errors.New("")
				body := ioutil.NopCloser(strings.NewReader("content"))
				err := client.Put("/", body)
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
