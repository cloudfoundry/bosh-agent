package monitaccess

import (
	"fmt"
	"os"
)

func EnableMonitAccess(command string, args []string) {
	fmt.Fprintf(os.Stderr, "enable-monit-access: not implemented for windows")
	os.Exit(1)
}
