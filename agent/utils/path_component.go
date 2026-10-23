package utils

import "strings"

// IsPathComponent reports whether a name is one file component: it is not
// empty, "." or "..", and contains no path separator or NUL byte. Colons are
// allowed, since some IaaS disk CIDs contain them.
func IsPathComponent(name string) bool {
	return name != "" && name != "." && name != ".." && !strings.ContainsAny(name, "/\\\x00")
}
