package action

import (
	"testing"
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/agent/applier/applyspec/fakes"
	"github.com/cloudfoundry/bosh-agent/v2/agent/script/scriptfakes"
	supervisorfakes "github.com/cloudfoundry/bosh-agent/v2/jobsupervisor/fakes"
	notifierfakes "github.com/cloudfoundry/bosh-agent/v2/notification/fakes"
	boshlog "github.com/cloudfoundry/bosh-utils/logger"
)

func TestFactoryTaskCancellationIsIsolated(t *testing.T) {
	script := &scriptfakes.FakeCancellableScript{}
	started := make(chan struct{})
	release := make(chan struct{})
	script.RunStub = func() error { close(started); <-release; return nil }
	provider := &scriptfakes.FakeJobScriptProvider{}
	provider.NewParallelScriptReturns(script)
	factory := concreteFactory{availableActions: map[string]Action{
		"drain":      NewDrain(notifierfakes.NewFakeNotifier(), &fakes.FakeV1Service{}, provider, &supervisorfakes.FakeJobSupervisor{}, boshlog.NewLogger(boshlog.LevelNone)),
		"run_errand": RunErrandAction{cancelCh: make(chan struct{}, 1)},
	}}
	old := createAction(t, factory, "drain")
	next := createAction(t, factory, "drain")
	if err := old.Cancel(); err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { _, err := next.(DrainAction).Run(DrainTypeShutdown); done <- err }()
	<-started
	select {
	case err := <-done:
		close(release)
		t.Fatalf("other task canceled this drain: %v", err)
	case <-time.After(30 * time.Millisecond):
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	if script.CancelCallCount() != 0 {
		t.Fatal("cancel reached a different task")
	}
	oldErrand := createAction(t, factory, "run_errand")
	nextErrand := createAction(t, factory, "run_errand")
	if err := oldErrand.Cancel(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-nextErrand.(RunErrandAction).cancelCh:
		t.Fatal("cancel reached a different errand task")
	default:
	}
}

func createAction(t *testing.T, factory concreteFactory, method string) Action {
	t.Helper()
	action, err := factory.Create(method)
	if err != nil {
		t.Fatal(err)
	}
	return action
}
