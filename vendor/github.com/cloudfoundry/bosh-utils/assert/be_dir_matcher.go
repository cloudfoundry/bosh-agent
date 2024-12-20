package assert

import (
	"fmt"
	"os"
)

type BeDir struct {
}

func (m BeDir) Match(actual interface{}) (bool, error) {
	path, ok := actual.(string)
	if !ok {
		return false, fmt.Errorf("`%s' is not a valid path", actual)
	}

	dir, err := os.Open(path)
	if err != nil {
		return false, fmt.Errorf("could not open `%s'", actual)
	}
	defer dir.Close()

	dirInfo, err := dir.Stat()
	if err != nil {
		return false, fmt.Errorf("could not stat `%s'", actual)
	}

	return dirInfo.IsDir(), nil
}

// FailureMessage (actual interface{}) (message string)
func (m BeDir) FailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected `%s' to be a directory", actual)
}

// NegatedFailureMessage (actual interface{}) (message string)
func (m BeDir) NegatedFailureMessage(actual interface{}) string {
	return fmt.Sprintf("Expected `%s' to not be a directory", actual)
}
