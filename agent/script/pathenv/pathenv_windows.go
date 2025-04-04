package pathenv

import (
	"os"
)

// Path returns the PATH environment variable for scripts.
func Path() string { return os.Getenv("PATH") }
