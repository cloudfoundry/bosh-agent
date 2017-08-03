// +build !windows

package evtlog

import "testing"

// This files exists only to make Ginkgo happy...

func TestIsGinkoHappy(t *testing.T) {
	if true == false {
		t.Fatal("WAT")
	}
}
