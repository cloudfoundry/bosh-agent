package mbus

import (
	"errors"
	"net"
	"reflect"
	"testing"
)

type dialFirewallHook func([]string) error

func (h dialFirewallHook) BeforeConnect(urls []string) error { return h(urls) }

func TestNATSDialsOnlyProtectedLiteralAddresses(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close() //nolint:errcheck
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	protected := false
	dialer := newFirewallDialer(dialFirewallHook(func(urls []string) error {
		// Simulate the original hostname now resolving to another endpoint. The
		// literal candidate must still be included in the firewall transaction.
		want := []string{"nats://logical.test:" + port, "nats://127.0.0.1:" + port}
		if !reflect.DeepEqual(urls, want) {
			t.Fatalf("firewall targets=%v", urls)
		}
		protected = true
		return nil
	}), []string{"nats://logical.test:" + port})
	lookups := 0
	dialer.lookupHost = func(string) ([]string, error) { lookups++; return []string{"127.0.0.1"}, nil }
	originalDial := dialer.dial
	dialer.dial = func(network, address string) (net.Conn, error) {
		if !protected {
			t.Fatal("TCP dial preceded firewall transaction")
		}
		if address != listener.Addr().String() {
			t.Fatalf("dialled unresolved endpoint %s", address)
		}
		return originalDial(network, address)
	}
	conn, err := dialer.Dial("tcp", "logical.test:"+port)
	if err != nil {
		t.Fatal(err)
	}
	if err := conn.Close(); err != nil {
		t.Fatal(err)
	}
	if lookups != 1 {
		t.Fatalf("endpoint was resolved %d times", lookups)
	}
}

func TestNATSDialFailsClosed(t *testing.T) {
	for _, lookupFailure := range []bool{false, true} {
		dialer := newFirewallDialer(dialFirewallHook(func([]string) error { return errors.New("firewall unavailable") }), nil)
		dialer.lookupHost = func(string) ([]string, error) {
			if lookupFailure {
				return nil, errors.New("DNS unavailable")
			}
			return []string{"127.0.0.1"}, nil
		}
		dialer.dial = func(string, string) (net.Conn, error) { t.Fatal("TCP attempted without protection"); return nil, nil }
		if _, err := dialer.Dial("tcp", "logical.test:4222"); err == nil {
			t.Fatal("failed protection was ignored")
		}
	}
}

func TestNATSReconnectRefreshesProtectionAndSupportsIPv6(t *testing.T) {
	attempt := 0
	protected := ""
	dialer := newFirewallDialer(dialFirewallHook(func(urls []string) error { protected = urls[len(urls)-1]; return nil }), nil)
	dialer.lookupHost = func(string) ([]string, error) {
		attempt++
		if attempt == 1 {
			return []string{"192.0.2.1"}, nil
		}
		return []string{"2001:db8::1"}, nil
	}
	dialer.dial = func(_ string, address string) (net.Conn, error) {
		if protected != "nats://"+address {
			t.Fatalf("unprotected reconnect target %s (%s)", address, protected)
		}
		left, right := net.Pipe()
		if err := right.Close(); err != nil {
			t.Fatal(err)
		}
		return left, nil
	}
	for i := 0; i < 2; i++ {
		conn, err := dialer.Dial("tcp", "logical.test:4222")
		if err != nil {
			t.Fatal(err)
		}
		if err := conn.Close(); err != nil {
			t.Fatal(err)
		}
	}
	if protected != "nats://[2001:db8::1]:4222" {
		t.Fatal(protected)
	}
}
