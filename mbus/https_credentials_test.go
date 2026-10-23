package mbus_test

import (
	"net/url"
	"testing"

	boshlog "github.com/cloudfoundry/bosh-utils/logger"

	"github.com/cloudfoundry/bosh-agent/v2/mbus"
	"github.com/cloudfoundry/bosh-agent/v2/settings"
)

func TestHTTPSDispatcherRequiresConfiguredCredentials(t *testing.T) {
	logger := boshlog.NewLogger(boshlog.LevelNone)
	for _, raw := range []string{"https://host:6868", "https://user@host:6868", "https://:password@host:6868", "https://user:@host:6868"} {
		address, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		if server, err := mbus.NewHTTPSDispatcher(address, settings.CertKeyPair{}, logger); err == nil || server != nil {
			t.Fatalf("accepted incomplete credentials: %s", raw)
		}
	}
	address, err := url.Parse("https://user:password@host:6868")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mbus.NewHTTPSDispatcher(address, settings.CertKeyPair{}, logger); err != nil {
		t.Fatal(err)
	}
}
