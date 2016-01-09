package windows_test

import (
	"fmt"
	"os/exec"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gbytes"
	"github.com/onsi/gomega/gexec"

	"testing"
)

func TestWindows(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Windows Suite")
}

var gnats *gexec.Session

var _ = BeforeSuite(func() {
	var err error

	command := exec.Command(fmt.Sprintf("../../bin/gnatsd-%s", runtime.GOOS), "-V")

	if gnats, err = gexec.Start(command, GinkgoWriter, GinkgoWriter); err != nil {
		Fail(fmt.Sprintf("Could not start the NATS Server, Err: %s", err))
	}
	Eventually(gnats.Err, 5*time.Second).Should(gbytes.Say("gnatsd is ready"))
})

var _ = AfterSuite(func() {
	gnats.Terminate()
})
