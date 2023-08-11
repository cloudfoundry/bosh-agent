package alert_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"testing"
)

func TestAlert(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Alert Suite")
}
