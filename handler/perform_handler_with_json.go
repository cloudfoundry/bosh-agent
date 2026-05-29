package handler

import (
	"encoding/json"
	"unicode/utf8"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

const (
	mbusHandlerLogTag       = "MBus Handler"
	responseMaxLengthErrMsg = "Response exceeded maximum allowed length"
	truncationMarker        = "\n...[truncated]...\n"
	UnlimitedResponseLength = -1
)

func PerformHandlerWithJSON(rawJSON []byte, handler Func, maxResponseLength int, logger boshlog.Logger) ([]byte, Request, error) {
	var request Request

	err := json.Unmarshal(rawJSON, &request)
	if err != nil {
		return []byte{}, request, bosherr.WrapError(err, "Unmarshalling JSON payload")
	}

	request.Payload = rawJSON

	response := handler(request)
	if response == nil {
		logger.Info(mbusHandlerLogTag, "Nil response returned from handler")
		return []byte{}, request, nil
	}

	respJSON, err := marshalResponse(response, maxResponseLength, logger)
	if err != nil {
		return respJSON, request, err
	}

	logger.Info(mbusHandlerLogTag, "Responding")
	logger.DebugWithDetails(mbusHandlerLogTag, "Payload", respJSON)

	return respJSON, request, nil
}

func BuildErrorWithJSON(msg string, logger boshlog.Logger) ([]byte, error) {
	response := NewExceptionResponse(bosherr.Error(msg))

	respJSON, err := json.Marshal(response)
	if err != nil {
		return respJSON, bosherr.WrapError(err, "Marshalling JSON")
	}

	logger.Info(mbusHandlerLogTag, "Building error", msg)

	return respJSON, nil
}

// truncateExceptionResponse truncates the message of excResp so that its JSON
// serialization fits within maxLength bytes. It preserves both the beginning and
// the end of the message, inserting truncationMarker in between, so that the
// high-level error context and the root cause are both visible.
// Returns the truncated response and true if successful, or false if it cannot
// be made to fit.
func truncateExceptionResponse(excResp exceptionResponse, maxLength int) (exceptionResponse, bool) {
	// Probe the fixed JSON overhead by marshaling just the truncationMarker as the
	// message. Because the `message` field uses omitempty, marshaling an empty struct
	// would omit the key entirely and undercount the overhead.
	// e.g. {"exception":{"message":"...[truncated]..."}} → fixedOverhead = len(probe) - len(marker)
	probe := exceptionResponse{}
	probe.Exception.Message = truncationMarker
	probeJSON, err := json.Marshal(probe)
	if err != nil {
		return exceptionResponse{}, false
	}

	fixedOverhead := len(probeJSON) - len(truncationMarker)
	maxMsgBytes := maxLength - fixedOverhead
	if maxMsgBytes <= len(truncationMarker) {
		return exceptionResponse{}, false
	}

	msg := excResp.Exception.Message

	// Short-circuit only if the actual marshaled response already fits. We can't
	// rely on len(msg) alone: JSON escaping (e.g. `"` → `\"`) can expand the
	// message past the limit even when its raw byte length is within maxMsgBytes.
	if fullJSON, err := json.Marshal(excResp); err == nil && len(fullJSON) <= maxLength {
		return excResp, true
	}

	// Split the available content bytes evenly between head and tail.
	// Tail gets the extra byte when contentBytes is odd, since the root cause
	// is typically found at the end of the error chain.
	//
	// We loop and re-marshal because json.Marshal expands escapable characters
	// (e.g. `"` → `\"`, `\` → `\\`, non-ASCII → `\uXXXX`), meaning the raw
	// byte estimate can undercount the actual JSON size. Each iteration scales
	// contentBytes down proportionally until the marshaled result fits.
	contentBytes := maxMsgBytes - len(truncationMarker)

	// Clamp to the message length so head/tail slicing never reads out of
	// bounds. This matters when a short-but-heavily-escaped message reaches the
	// loop: contentBytes can exceed len(msg), which would otherwise produce a
	// negative tail index.
	if contentBytes > len(msg) {
		contentBytes = len(msg)
	}

	for contentBytes > 0 {
		headBytes := contentBytes / 2
		tailBytes := contentBytes - headBytes

		// Trim head backwards to a valid UTF-8 rune boundary.
		head := msg[:headBytes]
		for len(head) > 0 && !utf8.ValidString(head) {
			head = head[:len(head)-1]
		}

		// Trim tail forwards past any UTF-8 continuation bytes (0x80–0xBF)
		// that would be invalid at the start of a string.
		tail := msg[len(msg)-tailBytes:]
		for len(tail) > 0 && tail[0]&0xC0 == 0x80 {
			tail = tail[1:]
		}

		candidate := excResp
		candidate.Exception.Message = head + truncationMarker + tail

		candidateJSON, err := json.Marshal(candidate)
		if err != nil {
			return exceptionResponse{}, false
		}
		if len(candidateJSON) <= maxLength {
			return candidate, true
		}

		// The JSON exceeded the limit due to character escaping. Reduce
		// contentBytes proportionally based on the observed expansion so the
		// next iteration converges toward the target size.
		newContentBytes := contentBytes * maxLength / len(candidateJSON)
		if newContentBytes >= contentBytes {
			// Integer division can round the proportional estimate back to the
			// current value when candidateJSON is only marginally over the
			// limit; step down by one to guarantee forward progress.
			contentBytes--
		} else {
			contentBytes = newContentBytes
		}
	}

	return exceptionResponse{}, false
}

func marshalResponse(response Response, maxResponseLength int, logger boshlog.Logger) ([]byte, error) {
	respJSON, err := json.Marshal(response)
	if err != nil {
		logger.Error(mbusHandlerLogTag, "Failed to marshal response: %s", err.Error())
		return respJSON, bosherr.WrapError(err, "Marshalling JSON response")
	}

	if maxResponseLength == UnlimitedResponseLength {
		return respJSON, nil
	}

	if len(respJSON) > maxResponseLength {
		shortened := response.Shorten()

		respJSON, err = json.Marshal(shortened)
		if err != nil {
			logger.Error(mbusHandlerLogTag, "Failed to marshal response: %s", err.Error())
			return respJSON, bosherr.WrapError(err, "Marshalling JSON response")
		}

		if len(respJSON) > maxResponseLength {
			if excResp, ok := shortened.(exceptionResponse); ok {
				if truncated, ok := truncateExceptionResponse(excResp, maxResponseLength); ok {
					respJSON, err = json.Marshal(truncated)
					if err == nil && len(respJSON) <= maxResponseLength {
						logger.Warn(mbusHandlerLogTag, "Response too large, exception message truncated to fit within %d bytes", maxResponseLength)
						return respJSON, nil
					}
				}
			}

			respJSON, err = BuildErrorWithJSON(responseMaxLengthErrMsg, logger)
			if err != nil {
				logger.Error(mbusHandlerLogTag, "Failed to build 'max length exceeded' response: %s", err.Error())
				return respJSON, bosherr.WrapError(err, "Building error")
			}
		}
	}

	return respJSON, nil
}
