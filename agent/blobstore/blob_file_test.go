package blobstore_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/cloudfoundry/bosh-agent/v2/agent/blobstore"
	"github.com/cloudfoundry/bosh-utils/crypto"
)

func TestBlobFilesStayInWorkDirectory(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	if err := os.WriteFile(outside, []byte("original"), 0600); err != nil {
		t.Fatal(err)
	}
	manager, err := blobstore.NewBlobManager(filepath.Join(root, "blobs"))
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"../outside", "..", ".", "", `..\outside`, "/outside", "C:outside"} {
		if err := manager.Write(id, strings.NewReader("replaced")); err == nil {
			t.Fatalf("accepted blob write %q", id)
		}
		if f, _, err := manager.Fetch(id); err == nil {
			_ = f.Close() //nolint:errcheck
			t.Fatalf("accepted blob read %q", id)
		}
		if err := manager.Delete(id); err == nil {
			t.Fatalf("accepted blob delete %q", id)
		}
		if manager.BlobExists(id) {
			t.Fatalf("accepted blob lookup %q", id)
		}
		if _, err := manager.GetPath(id, crypto.NewDigest(crypto.DigestAlgorithmSHA1, "")); err == nil {
			t.Fatalf("accepted blob path %q", id)
		}
	}
	got, err := os.ReadFile(outside)
	if err != nil || string(got) != "original" {
		t.Fatal("outside file changed")
	}
	if err := manager.Write("ordinary-id", strings.NewReader("content")); err != nil {
		t.Fatal(err)
	}
	if !manager.BlobExists("ordinary-id") {
		t.Fatal("ordinary blob missing")
	}
}
