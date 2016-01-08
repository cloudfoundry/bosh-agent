package handler_test

import (
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	boshHandler "github.com/aemengo/bosh-agent/handler"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

var _ = Describe("PerformHandlerWithJSON", func() {
	It("defaults to an 'unknown sender' when sender is provided in the request", func() {
		rawJSON := []byte(`{"method":"ping","arguments":[]}`)
		handlerFunc := func(req boshHandler.Request) (resp boshHandler.Response) { return boshHandler.NewValueResponse("pong") }
		responseMaxLength := 1024 * 1024
		logger := boshlog.NewLogger(boshlog.LevelNone)

		_, request, _ := boshHandler.PerformHandlerWithJSON(
			rawJSON,
			handlerFunc,
			responseMaxLength,
			logger,
		)

		Expect(request.ReplyTo).To(Equal("unknown-sender"))
	})

	It("identifies the sender when the sender is provided in the request", func() {
		rawJSON := []byte(`{"method":"ping","arguments":[],"reply_to":"director.987-654-321"}`)
		handlerFunc := func(req boshHandler.Request) (resp boshHandler.Response) { return boshHandler.NewValueResponse("pong") }
		responseMaxLength := 1024 * 1024
		logger := boshlog.NewLogger(boshlog.LevelNone)

		_, request, _ := boshHandler.PerformHandlerWithJSON(
			rawJSON,
			handlerFunc,
			responseMaxLength,
			logger,
		)

		Expect(request.ReplyTo).To(Equal("director.987-654-321"))
	})
})
