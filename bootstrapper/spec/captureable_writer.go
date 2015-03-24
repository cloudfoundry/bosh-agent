package spec

import (
	"bytes"
	"io"
	"regexp"
)

type CapturableWriter interface {
	Write(p []byte) (int, error)
	Suppress(pattern string, block func())
	Ignore(pattern string) *regexp.Regexp
	Capture(pattern string)
	Captured() string
}

type capturableWriter struct {
	out             io.Writer
	capturePatterns []*regexp.Regexp
	ignorePatterns  []*regexp.Regexp
	captured        bytes.Buffer
}

func NewCapturableWriter(out io.Writer) CapturableWriter {
	return &capturableWriter{
		out:             out,
		capturePatterns: []*regexp.Regexp{},
		ignorePatterns:  []*regexp.Regexp{},
	}
}

func (writer *capturableWriter) Write(p []byte) (int, error) {
	for _, pattern := range writer.capturePatterns {
		if pattern.Match(p) {
			return writer.captured.Write(p)
		}
	}
	for _, pattern := range writer.ignorePatterns {
		if pattern.Match(p) {
			return 0, nil
		}
	}
	return writer.out.Write(p)
}

func (writer *capturableWriter) Suppress(pattern string, block func()) {
	regexp := writer.Ignore(pattern)
	block()
	writer.unIgnore(regexp)
}

func (writer *capturableWriter) Ignore(pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	writer.ignorePatterns = append(writer.ignorePatterns, re)
	return re
}

func (writer *capturableWriter) unIgnore(re *regexp.Regexp) {
	newIgnorePatterns := []*regexp.Regexp{}
	for _, existing := range writer.ignorePatterns {
		if existing != re {
			newIgnorePatterns = append(newIgnorePatterns, existing)
		}
	}

	writer.ignorePatterns = newIgnorePatterns
}

func (writer *capturableWriter) Capture(pattern string) {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	writer.capturePatterns = append(writer.capturePatterns, re)
}

func (writer *capturableWriter) Captured() string {
	return writer.captured.String()
}
