package main

import (
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

func timeoutEnv(name string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(os.Getenv(name))
	if raw == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(raw)
	if err != nil || duration <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration such as 10s, 60s or 2m", name)
	}
	return duration, nil
}
func upstreamTransport() (*http.Transport, error) {
	connect, err := timeoutEnv("UPSTREAM_CONNECT_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	tls, err := timeoutEnv("UPSTREAM_TLS_TIMEOUT", 10*time.Second)
	if err != nil {
		return nil, err
	}
	headers, err := timeoutEnv("UPSTREAM_HEADER_TIMEOUT", 30*time.Second)
	if err != nil {
		return nil, err
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.DialContext = (&net.Dialer{Timeout: connect, KeepAlive: 30 * time.Second}).DialContext
	transport.TLSHandshakeTimeout = tls
	transport.ResponseHeaderTimeout = headers
	return transport, nil
}
