package models_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	. "github.com/cloudfoundry/bosh-agent/v2/agent/applier/models"
)

var _ = Describe("Source", func() {
	Describe("String", func() {
		It("redacts the signed URL and blobstore headers", func() {
			source := Source{
				BlobstoreID:      "fake-blob-id",
				PathInArchive:    "fake-path",
				SignedURL:        "https://example.com/blob?X-Amz-Signature=supersecret",
				BlobstoreHeaders: map[string]string{"Authorization": "Basic dopeToken"},
			}

			str := source.String()
			Expect(str).ToNot(ContainSubstring("supersecret"))
			Expect(str).ToNot(ContainSubstring("dopeToken"))
			Expect(str).To(ContainSubstring("SignedURL:<redacted>"))
			Expect(str).To(ContainSubstring("fake-blob-id"))
		})

		It("keeps non-secret fields visible while redacting the signed URL", func() {
			source := Source{BlobstoreID: "fake-blob-id"}
			str := source.String()
			Expect(str).To(ContainSubstring("fake-blob-id"))
			// SignedURL is a redact.Secret, so it always renders redacted.
			Expect(str).To(ContainSubstring("SignedURL:<redacted>"))
		})

		It("is honoured when a Job containing the source is formatted with %v", func() {
			job := Job{
				Name: "fake-job",
				Packages: []Package{{
					Name:   "fake-pkg",
					Source: Source{SignedURL: "https://example.com/blob?X-Amz-Signature=supersecret"},
				}},
				Source: Source{SignedURL: "https://example.com/job?X-Amz-Signature=alsosecret"},
			}

			out := fmt.Sprintf("%v", job)
			Expect(out).ToNot(ContainSubstring("supersecret"))
			Expect(out).ToNot(ContainSubstring("alsosecret"))
			Expect(out).To(ContainSubstring("<redacted>"))
		})
	})
})
