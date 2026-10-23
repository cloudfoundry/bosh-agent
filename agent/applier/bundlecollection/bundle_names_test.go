package bundlecollection

import "testing"

func TestBundleNamesCheckedBeforePaths(t *testing.T) {
	for _, name := range []string{"../outside", "..", `..\outside`, "/outside"} {
		collection := FileBundleCollection{}
		definition := fileBundleDefinition{name: name, version: "1"}
		if _, err := collection.Get(definition); err == nil {
			t.Fatalf("accepted bundle %q", name)
		}
		if _, err := collection.getDigested(definition); err == nil {
			t.Fatalf("accepted installed bundle %q", name)
		}
	}
	for _, name := range []string{"--checkpoint=1", "../outside", "/outside"} {
		if _, err := (FileBundle{}).Install("archive", name); err == nil {
			t.Fatalf("accepted bundle member %q", name)
		}
	}
}
