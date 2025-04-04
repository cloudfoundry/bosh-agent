package jobsupervisor_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestJobsupervisor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Job Supervisor Suite")
}
