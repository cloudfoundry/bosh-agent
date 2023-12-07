//go:build !linux

package main

import (
	"fmt"
)

func readStemcellSlug() (string, string, string, error) {
	return "", "", "", fmt.Errorf("readStemcellSlug: not implemented")
}
