package fakes

type FakeKernelIPv6 struct {
	Enabled   bool
	EnableErr error
}

func (net *FakeKernelIPv6) Enable(stopCh <-chan struct{}) error {
	net.Enabled = true
	return net.EnableErr
}
