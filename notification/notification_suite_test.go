package notification_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestNotification(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Notification Suite")
}
