package mbus

import (
	"fmt"
	"net"
	"time"

	"github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
)

// firewallDialer installs rules for the literal addresses it will dial. The
// logical NATS URL remains intact for TLS verification and cluster selection.
type firewallDialer struct {
	hook       firewall.NatsFirewallHook
	mbusURLs   []string
	lookupHost func(string) ([]string, error)
	dial       func(string, string) (net.Conn, error)
}

func newFirewallDialer(hook firewall.NatsFirewallHook, urls []string) *firewallDialer {
	dialer := &net.Dialer{Timeout: 2 * time.Second}
	return &firewallDialer{hook: hook, mbusURLs: append([]string(nil), urls...), lookupHost: net.LookupHost, dial: dialer.Dial}
}

func (d *firewallDialer) Dial(network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	addresses := []string{host}
	if net.ParseIP(host) == nil {
		addresses, err = d.lookupHost(host)
		if err != nil {
			return nil, fmt.Errorf("resolving NATS endpoint: %w", err)
		}
	}
	if len(addresses) == 0 {
		return nil, fmt.Errorf("NATS endpoint resolved to no addresses")
	}
	urls := append([]string(nil), d.mbusURLs...)
	endpoints := make([]string, 0, len(addresses))
	for _, ip := range addresses {
		if net.ParseIP(ip) == nil {
			return nil, fmt.Errorf("NATS resolver returned an invalid IP address")
		}
		endpoint := net.JoinHostPort(ip, port)
		endpoints = append(endpoints, endpoint)
		// Include all configured endpoints for existing HA protection, and also
		// pin every actual dial candidate even if a later DNS lookup changes.
		urls = append(urls, "nats://"+endpoint)
	}
	if err := d.hook.BeforeConnect(urls); err != nil {
		return nil, fmt.Errorf("protecting NATS endpoint: %w", err)
	}
	for _, endpoint := range endpoints {
		var conn net.Conn
		conn, err = d.dial(network, endpoint)
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}
