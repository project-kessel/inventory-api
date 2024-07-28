package util

import (
	"crypto/tls"
	"net"
	"net/http"
	"time"
)

func NewClient(insecure bool) *http.Client {
	if insecure {
		// like http.DefaultTransport but with InsecureSkipVerify: true
		transport := &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: insecure,
			},
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   30 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			IdleConnTimeout:       90 * time.Second,
			TLSHandshakeTimeout:   10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
		}
		return &http.Client{
			Transport: transport,
		}
	}
	return http.DefaultClient
}
