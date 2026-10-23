package compiler

import "testing"

func TestCompileNamesCheckedBeforeWork(t *testing.T) {
	for _, name := range []string{"../outside", "..", `..\outside`, "/outside", ""} {
		if _, _, err := (concreteCompiler{}).Compile(Package{Name: name}, nil); err == nil {
			t.Fatalf("accepted package %q", name)
		}
	}
}
