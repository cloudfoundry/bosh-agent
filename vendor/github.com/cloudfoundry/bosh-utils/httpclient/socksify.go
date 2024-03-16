package httpclient

import (
	"context"
	"net"
	"net/url"
	"os"
	"strings"
	"sync"

	bosherr "github.com/cloudfoundry/bosh-utils/errors"
	proxy "github.com/cloudfoundry/socks5-proxy"

	goproxy "golang.org/x/net/proxy"
)

type ProxyDialer interface {
	Dialer(string, string, string) (proxy.DialFunc, error)
}

type DialContextFunc func(ctx context.Context, network, address string) (net.Conn, error)

func (f DialContextFunc) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return f(ctx, network, address)
}

func SOCKS5DialContextFuncFromEnvironment(origDialer *net.Dialer, socks5Proxy ProxyDialer) DialContextFunc {
	allProxy := os.Getenv("BOSH_ALL_PROXY")
	if len(allProxy) == 0 {
		return origDialer.DialContext
	}

	if strings.HasPrefix(allProxy, "ssh+") {
		allProxy = strings.TrimPrefix(allProxy, "ssh+")

		proxyURL, err := url.Parse(allProxy)
		if err != nil {
			return errorDialFunc(err, "Parsing BOSH_ALL_PROXY url")
		}

		queryMap, err := url.ParseQuery(proxyURL.RawQuery)
		if err != nil {
			return errorDialFunc(err, "Parsing BOSH_ALL_PROXY query params")
		}

		username := ""
		if proxyURL.User != nil {
			username = proxyURL.User.Username()
		}

		proxySSHKeyPath := queryMap.Get("private-key")
		if proxySSHKeyPath == "" {
			return errorDialFunc(
				bosherr.Error("Required query param 'private-key' not found"),
				"Parsing BOSH_ALL_PROXY query params",
			)
		}

		proxySSHKey, err := os.ReadFile(proxySSHKeyPath)
		if err != nil {
			return errorDialFunc(err, "Reading private key file for SOCKS5 Proxy")
		}

		var (
			dialer proxy.DialFunc
			mut    sync.RWMutex
		)
		return func(ctx context.Context, network, address string) (net.Conn, error) {
			mut.RLock()
			haveDialer := dialer != nil
			mut.RUnlock()

			if haveDialer {
				return dialer(network, address)
			}

			mut.Lock()
			defer mut.Unlock()
			if dialer == nil {
				proxyDialer, err := socks5Proxy.Dialer(username, string(proxySSHKey), proxyURL.Host)
				if err != nil {
					return nil, bosherr.WrapErrorf(err, "Creating SOCKS5 dialer")
				}
				dialer = proxyDialer
			}
			return dialer(network, address)
		}
	}

	proxyURL, err := url.Parse(allProxy)
	if err != nil {
		return errorDialFunc(err, "Parsing BOSH_ALL_PROXY url")
	}

	proxy, err := goproxy.FromURL(proxyURL, origDialer)
	if err != nil {
		return errorDialFunc(err, "Parsing BOSH_ALL_PROXY url")
	}

	perHost := goproxy.NewPerHost(proxy, origDialer)

	noProxy := os.Getenv("NO_PROXY")
	if len(noProxy) == 0 {
		noProxy = os.Getenv("no_proxy")
	}

	if len(noProxy) != 0 {
		perHost.AddFromString(noProxy)
	}

	return perHost.DialContext
}

func errorDialFunc(err error, cause string) DialContextFunc {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		return nil, bosherr.WrapError(err, cause)
	}
}
