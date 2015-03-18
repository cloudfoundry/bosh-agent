package bootstrapper

import (
	"os/exec"
	"syscall"
)

func panicIfError(err error) {
	if err != nil {
		panic(err)
	}
}

func getExitStatus(err error) int {
	if err == nil {
		return 0
	}

	if exiterr, ok := err.(*exec.ExitError); ok {
		if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
			return status.ExitStatus()
		}
	}
	panic(err)
}
