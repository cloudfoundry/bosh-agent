// +build windows

package fs

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func MakePath() string {
	volume := os.TempDir() + string(filepath.Separator)
	buf := bytes.NewBufferString(volume)
	for i := 0; i < 2; i++ {
		for i := byte('A'); i <= 'Z'; i++ {
			buf.Write(bytes.Repeat([]byte{i}, 4))
			buf.WriteRune(filepath.Separator)
		}
	}
	return filepath.Clean(buf.String())
}

func TestMkdirAll(t *testing.T) {
	path := MakePath()
	defer os.RemoveAll(path)
	err := MkdirAll(MakePath(), 0755)
	if err != nil {
		t.Fatalf("TestMkdirAll: %s", err)
	}
	if _, err := Stat(path); err != nil {
		t.Fatalf("TestMkdirAll: Stat failed %s", err)
	}
	// Make sure the handling of long paths is case-insensitive
	if _, err := Stat(strings.ToLower(path)); err != nil {
		t.Fatalf("TestMkdirAll: Stat failed %s", err)
	}
	if err := os.RemoveAll(path); err != nil {
		t.Fatalf("TestMkdirAll: RemoveAll %s", err)
	}
}

func TestRemoveAll(t *testing.T) {
	path := MakePath()
	fmt.Println(path)
	defer os.RemoveAll(path)
}
