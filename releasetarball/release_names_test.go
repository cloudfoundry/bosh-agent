package releasetarball

import (
	"testing"

	"github.com/cloudfoundry/bosh-cli/v7/release/manifest"
)

func TestCompiledReleaseNamesCheckedBeforeOutput(t *testing.T) {
	for _, m := range []manifest.Manifest{{Name: "../outside", Version: "1"}, {Name: "release", Version: "../../../outside"}} {
		if _, err := writeCompiledRelease(m, t.TempDir(), "ubuntu/x", t.TempDir(), "", nil, nil); err == nil {
			t.Fatal("accepted invalid compiled release filename")
		}
	}
}
