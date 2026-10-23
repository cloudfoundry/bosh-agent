package mbus

import (
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	boshagentblobstore "github.com/cloudfoundry/bosh-agent/v2/agent/blobstore"
	"github.com/cloudfoundry/bosh-agent/v2/platform/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

func TestBlobUploadOutlastsServerReadTimeout(t *testing.T) {
	blobManager, err := boshagentblobstore.NewBlobManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	address, err := url.Parse("https://user:password@localhost:6868")
	if err != nil {
		t.Fatal(err)
	}
	handler, err := NewHTTPSHandler(address, settings.CertKeyPair{}, blobManager, boshlog.NewLogger(boshlog.LevelNone), fakes.NewFakeAuditLogger())
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewUnstartedServer(http.HandlerFunc(handler.blobsHandler()))
	server.Config.ReadTimeout = 200 * time.Millisecond
	server.Start()
	defer server.Close()

	body, writer := io.Pipe()
	go func() {
		for i := 0; i < 6; i++ {
			if _, err := writer.Write([]byte("chunk")); err != nil {
				return
			}
			time.Sleep(100 * time.Millisecond)
		}
		_ = writer.Close() //nolint:errcheck
	}()
	request, err := http.NewRequest("PUT", server.URL+"/blobs/slow-blob", body)
	if err != nil {
		t.Fatal(err)
	}
	response, err := server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close() //nolint:errcheck
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("status %d", response.StatusCode)
	}
	file, _, err := blobManager.Fetch("slow-blob")
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close() //nolint:errcheck
	contents, err := io.ReadAll(file)
	if err != nil {
		t.Fatal(err)
	}
	if string(contents) != strings.Repeat("chunk", 6) {
		t.Fatalf("contents %q", contents)
	}
}
