package main

import (
	"bytes"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("newCompileTarballOptions", func() {
	It("parses --exclude-compile-time-packages", func() {
		options, err := newCompileTarballOptions("compile", []string{"--exclude-compile-time-packages", "--output-directory", "/x", "a.tgz"})
		Expect(err).NotTo(HaveOccurred())
		Expect(options.ExcludeCompileTimePackages).To(BeTrue())
		Expect(options.OutputDirectory).To(Equal("/x"))
		Expect(options.SourceReleases).To(Equal([]string{"a.tgz"}))
	})

	It("leaves ExcludeCompileTimePackages false without the flag", func() {
		options, err := newCompileTarballOptions("compile", []string{"a.tgz"})
		Expect(err).NotTo(HaveOccurred())
		Expect(options.ExcludeCompileTimePackages).To(BeFalse())
	})
})

var _ = Describe("compile usage", func() {
	It("describes the trimmed output", func() {
		var options CompileTarballOptions
		flags := newCompileTarballFlagSet("compile", &options)
		var usage bytes.Buffer
		flags.SetOutput(&usage)
		flags.Usage()

		for _, want := range []string{"+trimmed", ".trimmed", "job.MF", "roll back", "stemcell"} {
			Expect(usage.String()).To(ContainSubstring(want))
		}
	})
})
