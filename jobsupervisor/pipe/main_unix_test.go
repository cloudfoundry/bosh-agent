// +build !windows

package main

import (
	"testing"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

// This files exists only to make Ginkgo happy...

func TestWinswPipe(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "WinswPipe Suite")
}
