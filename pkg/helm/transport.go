package helm

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"

	"golang.org/x/net/http/httpproxy"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
)

// ErrInvalidCA indicates that a CA bundle could not be parsed.
var ErrInvalidCA = errors.New("invalid certificate authority")

// proxyTransport creates an [*http.Transport] that routes requests through
// the proxy configured on the [Client]. It returns nil when no proxy is
// configured, in which case default transports (which honor proxy
// environment variables) should be used instead.
func (c *Client) proxyTransport() *http.Transport {
	if c.Proxy == "" && c.NoProxy == "" {
		return nil
	}

	cfg := &httpproxy.Config{
		HTTPProxy:  c.Proxy,
		HTTPSProxy: c.Proxy,
		NoProxy:    c.NoProxy,
	}
	proxyFunc := cfg.ProxyFunc()

	tr, ok := http.DefaultTransport.(*http.Transport)
	if ok {
		tr = tr.Clone()
	} else {
		tr = &http.Transport{}
	}

	tr.Proxy = func(req *http.Request) (*url.URL, error) {
		return proxyFunc(req.URL)
	}

	return tr
}

// getters returns the getter providers used for chart downloads. When a
// proxy is configured on the [Client], the getters are constructed with a
// transport that routes through the proxy and carries the given TLS
// configuration, since a custom transport takes precedence over TLS-related
// getter options.
func (c *Client) getters(certFile, keyFile, caFile string, insecureSkipVerify bool) (getter.Providers, error) {
	newHTTPGetter := getter.NewHTTPGetter
	newOCIGetter := getter.NewOCIGetter

	if c.transport != nil {
		tr := c.transport.Clone()

		tlsConf, err := newTLSConfig(certFile, keyFile, caFile, insecureSkipVerify)
		if err != nil {
			return nil, fmt.Errorf("configure tls: %w", err)
		}

		if tlsConf != nil {
			tr.TLSClientConfig = tlsConf
		}

		withTransport := func(constructor getter.Constructor) getter.Constructor {
			return func(options ...getter.Option) (getter.Getter, error) {
				return constructor(append(options, getter.WithTransport(tr))...)
			}
		}

		newHTTPGetter = withTransport(newHTTPGetter)
		newOCIGetter = withTransport(newOCIGetter)
	}

	return getter.Providers{
		{Schemes: []string{"http", "https"}, New: newHTTPGetter},
		{Schemes: []string{registry.OCIScheme}, New: newOCIGetter},
	}, nil
}

// newTLSConfig creates a [*tls.Config] from repository TLS settings. It
// returns nil when no TLS settings are provided.
func newTLSConfig(certFile, keyFile, caFile string, insecureSkipVerify bool) (*tls.Config, error) {
	if certFile == "" && keyFile == "" && caFile == "" && !insecureSkipVerify {
		return nil, nil //nolint:nilnil // nil config means use defaults.
	}

	cfg := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify, //nolint:gosec // G402: user opt-in via repository config.
		MinVersion:         tls.VersionTLS12,
	}

	if certFile != "" && keyFile != "" {
		cert, err := tls.LoadX509KeyPair(certFile, keyFile)
		if err != nil {
			return nil, fmt.Errorf("load client certificate: %w", err)
		}

		cfg.Certificates = []tls.Certificate{cert}
	}

	if caFile != "" {
		pem, err := os.ReadFile(caFile) //nolint:gosec // G304: path provided via repository config.
		if err != nil {
			return nil, fmt.Errorf("read ca file: %w", err)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("%w: %q", ErrInvalidCA, caFile)
		}

		cfg.RootCAs = pool
	}

	return cfg, nil
}
