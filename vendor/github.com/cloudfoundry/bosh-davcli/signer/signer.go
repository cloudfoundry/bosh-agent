package signer

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"time"
)

type Signer interface {
	GenerateSignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error)
}

type signer struct {
	secret string
}

func NewSigner(secret string) Signer {
	return &signer{
		secret: secret,
	}
}

func (s *signer) generateSignature(prefixedBlobID, verb string, timeStamp time.Time, expires int) string {
	verb = strings.ToUpper(verb)
	signature := fmt.Sprintf("%s%s%d%d", verb, prefixedBlobID, timeStamp.Unix(), expires)
	hmac := hmac.New(sha256.New, []byte(s.secret))
	hmac.Write([]byte(signature))
	sigBytes := hmac.Sum(nil)
	return base64.URLEncoding.WithPadding(base64.NoPadding).EncodeToString(sigBytes)
}

func (s *signer) GenerateSignedURL(endpoint, prefixedBlobID, verb string, timeStamp time.Time, expiresAfter time.Duration) (string, error) {
	verb = strings.ToUpper(verb)
	if verb != "GET" && verb != "PUT" {
		return "", fmt.Errorf("action not implemented: %s. Available actions are 'GET' and 'PUT'", verb)
	}

	endpoint = strings.TrimSuffix(endpoint, "/")
	expiresAfterSeconds := int(expiresAfter.Seconds())
	signature := s.generateSignature(prefixedBlobID, verb, timeStamp, expiresAfterSeconds)

	return fmt.Sprintf("%s/%s?st=%s&ts=%d&e=%d", endpoint, prefixedBlobID, signature, timeStamp.Unix(), expiresAfterSeconds), nil
}
