//go:build windows

package net

import (
	gonet "net"
	gonetIP "net/netip"
	gonetURL "net/url"
	"strconv"
	"strings"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	"golang.org/x/sys/windows"
	"inet.af/wf"
)

func SetupNatsFirewall(mbus string) error {
	// return early if we get an empty string for mbus. this is the case when the network for the host is just getting setup or in unit tests.
	if mbus == "" || strings.HasPrefix(mbus, "https://") {
		return nil
	}
	natsURI, err := gonetURL.Parse(mbus)
	if err != nil || natsURI.Hostname() == "" {
		return bosherr.WrapError(err, "Error parsing MbusURL")
	}
	session, err := wf.New(&wf.Options{
		Name:    "Windows Firewall Session for Bosh Agent",
		Dynamic: true, // setting this to true will create an ephemeral FW Rule that lasts as long as the Agent Process runs.
	})
	if err != nil {
		return bosherr.WrapError(err, "Getting windows firewall session")
	}
	guid, err := windows.GenerateGUID()
	if err != nil {
		return bosherr.WrapError(err, "Generating windows guid")
	}
	sublayerID := wf.SublayerID(guid)

	err = session.AddSublayer(&wf.Sublayer{
		ID:     sublayerID,
		Name:   "Default route killswitch",
		Weight: 0xffff, // the highest possible weight so all traffic to pass this Layer
	})
	if err != nil {
		return bosherr.WrapError(err, "Adding windows firewall session sublayer")
	}
	// These layers are the Input / Output stages of the Windows Firewall.
	// https://docs.microsoft.com/en-us/windows/win32/fwp/application-layer-enforcement--ale-
	layers := []wf.LayerID{
		wf.LayerALEAuthRecvAcceptV4,
		// wf.LayerALEAuthRecvAcceptV6,  //#TODO: Do we need v6?
		wf.LayerALEAuthConnectV4,
		// wf.LayerALEAuthConnectV6,	//#TODO: Do we need v6?
	}

	// The Windows app id will be used to create a conditional exception for the block outgoing nats rule.
	appID, err := wf.AppID("C:\\bosh\\bosh-agent.exe") // Could this ever be somewhere else?
	if err != nil {
		return bosherr.WrapError(err, "Getting the windows app id for bosh-agent.exe")
	}

	// We could technically have a hostname in the agent-settings.json for the mbus.
	// If it is already an IP LookupHost will return an Array containing the IP addr.
	natsIPs, err := gonet.LookupHost(natsURI.Hostname())
	if err != nil {
		return bosherr.WrapError(err, "Resolving mbus ips from settings")
	}
	natsPort, err := strconv.Atoi(natsURI.Port())
	if err != nil {
		return bosherr.WrapError(err, "Parsing Nats Port from URI")
	}
	for _, natsIPString := range natsIPs {
		natsIP, err := gonetIP.ParseAddr(natsIPString)
		if err != nil {
			return bosherr.WrapError(err, "Parsing mbus ip")
		}
		// The Firewall rule will check if the Target IP is within natsIp/32 Range, thus matching exactly the NatsIP
		natsIPCidr, err := natsIP.Prefix(32)
		if err != nil {
			return bosherr.WrapError(err, "Converting ip address to cidr annotation")
		}
		for _, layer := range layers {
			guid, err := windows.GenerateGUID()
			if err != nil {
				return bosherr.WrapError(err, "Generating windows guid")
			}

			err = session.AddRule(&wf.Rule{
				ID:       wf.RuleID(guid),
				Name:     "Allow traffic to remote bosh nats for bosh-agent app id, block everything else",
				Layer:    layer,
				Sublayer: sublayerID,
				Weight:   1000,
				Conditions: []*wf.Match{
					// Block traffic to natsIp:natsPort
					{
						Field: wf.FieldIPRemoteAddress,
						Op:    wf.MatchTypePrefix,
						Value: natsIPCidr,
					},
					{
						Field: wf.FieldIPRemotePort,
						Op:    wf.MatchTypeEqual,
						Value: uint16(natsPort),
					},
					// Exemption for bosh-agent appID
					{
						Field: wf.FieldALEAppID,
						Op:    wf.MatchTypeNotEqual,
						Value: appID,
					},
				},
				Action: wf.ActionBlock,
			})
			if err != nil {
				return bosherr.WrapError(err, "Adding firewall rule to limit remote nats access to bosh-agent")
			}
		}
	}
	return nil
}
