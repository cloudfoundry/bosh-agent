package handler_test

import (
	"encoding/json"
	"errors"
	"strings"
	"unicode/utf8"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	. "github.com/cloudfoundry/bosh-agent/v2/handler"
)

var _ = Describe("PerformHandlerWithJSON", func() {
	var logger boshlog.Logger

	BeforeEach(func() {
		logger = boshlog.NewLogger(boshlog.LevelNone)
	})

	Context("when the response is within the size limit", func() {
		It("returns the full response unchanged", func() {
			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"ping","arguments":[],"reply_to":"fake-reply"}`),
				func(req Request) Response {
					return NewValueResponse("pong")
				},
				1024,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respBytes)).To(Equal(`{"value":"pong"}`))
		})
	})

	Context("when the response exceeds the size limit", func() {
		buildLargeExceptionHandler := func(msg string) Func {
			return func(req Request) Response {
				return NewExceptionResponse(errors.New(msg))
			}
		}

		It("truncates the exception message preserving head and tail around the marker", func() {
			const maxLen = 150
			// Construct a message with a distinct head and tail so we can verify both survive.
			head := strings.Repeat("H", 500)
			tail := strings.Repeat("T", 500)
			msg := head + strings.Repeat("M", 500) + tail

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(msg),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically("<=", maxLen))

			var parsed map[string]map[string]string
			Expect(json.Unmarshal(respBytes, &parsed)).To(Succeed())
			result := parsed["exception"]["message"]

			Expect(result).To(ContainSubstring("\n...[truncated]...\n"), "truncation marker should be on its own line")
			Expect(result).To(HavePrefix("H"), "should preserve the beginning of the message")
			Expect(result).To(HaveSuffix("T"), "should preserve the end of the message")
			Expect(utf8.ValidString(result)).To(BeTrue(), "truncated message should be valid UTF-8")
		})

		It("produces JSON that is exactly at or under the limit", func() {
			const maxLen = 200

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(strings.Repeat("x", maxLen*5)),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically("<=", maxLen))
		})

		It("falls back to the generic error when response cannot be truncated to fit", func() {
			// maxLen is so small that even the truncation marker alone won't fit.
			const maxLen = 10

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(strings.Repeat("x", 1000)),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(respBytes)).To(ContainSubstring("Response exceeded maximum allowed length"))
		})

		It("preserves valid UTF-8 when truncating a multi-byte message", func() {
			const maxLen = 100
			// Build a message entirely of 3-byte UTF-8 runes ('€' = 0xE2 0x82 0xAC).
			multiByteMsg := strings.Repeat("€", maxLen)

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(multiByteMsg),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically("<=", maxLen))

			var parsed map[string]map[string]string
			Expect(json.Unmarshal(respBytes, &parsed)).To(Succeed())
			result := parsed["exception"]["message"]
			Expect(utf8.ValidString(result)).To(BeTrue(), "result should be valid UTF-8")
		})

		It("respects the limit when the message contains JSON-escapable characters", func() {
			// `"` marshals to `\"` (2 bytes), so a naive byte-count would produce
			// JSON roughly twice as large as expected. The handler must either
			// successfully truncate within the limit or fall back to the generic
			// error — it must never return oversized or invalid JSON.
			const maxLen = 200
			escapableMsg := strings.Repeat(`"`, maxLen*10)

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(escapableMsg),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically("<=", maxLen), "response must not exceed the size limit")

			var parsed map[string]interface{}
			Expect(json.Unmarshal(respBytes, &parsed)).To(Succeed(), "response must be valid JSON")

			// Accept either a truncated exception message or the generic fallback.
			if exc, ok := parsed["exception"].(map[string]interface{}); ok {
				msg, _ := exc["message"].(string)
				Expect(utf8.ValidString(msg)).To(BeTrue(), "exception message must be valid UTF-8")
			}
		})

		It("truncates rather than falling back when the raw message fits but its escaped form does not", func() {
			// The message's raw byte length fits within the limit, but every byte
			// is an escapable quote, so its marshaled (escaped) size is ~2x larger
			// and exceeds the limit. The handler must still produce a truncated
			// exception message (with the marker), not the generic fallback.
			const maxLen = 120
			escapableMsg := strings.Repeat(`"`, maxLen-40)

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(escapableMsg),
				maxLen,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically("<=", maxLen), "response must not exceed the size limit")
			Expect(string(respBytes)).NotTo(ContainSubstring("Response exceeded maximum allowed length"), "should truncate, not fall back to the generic error")

			var parsed map[string]map[string]string
			Expect(json.Unmarshal(respBytes, &parsed)).To(Succeed(), "response must be valid JSON")
			Expect(parsed["exception"]["message"]).To(ContainSubstring("...[truncated]..."), "should contain truncation marker")
		})

		It("does not truncate when UnlimitedResponseLength is set", func() {
			const bigMsgSize = 2 * 1024 * 1024 // 2 MB

			respBytes, _, err := PerformHandlerWithJSON(
				[]byte(`{"method":"big","arguments":[],"reply_to":"fake-reply"}`),
				buildLargeExceptionHandler(strings.Repeat("x", bigMsgSize)),
				UnlimitedResponseLength,
				logger,
			)
			Expect(err).NotTo(HaveOccurred())
			Expect(len(respBytes)).To(BeNumerically(">", bigMsgSize))
			Expect(string(respBytes)).NotTo(ContainSubstring("...[truncated]..."))
		})
	})
})
