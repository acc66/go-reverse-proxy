package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestTimeoutConfiguration(t *testing.T) {
	for _, name := range []string{"UPSTREAM_CONNECT_TIMEOUT", "UPSTREAM_TLS_TIMEOUT", "UPSTREAM_HEADER_TIMEOUT"} {
		t.Setenv(name, "")
		value, err := timeoutEnv(name, 30*time.Second)
		if err != nil || value != 30*time.Second {
			t.Fatal("default", value, err)
		}
		for _, raw := range []string{"0", "0s", "-1s", "60", "invalid"} {
			t.Setenv(name, raw)
			if _, err := upstreamTransport(); err == nil {
				t.Fatalf("accepted %s=%s", name, raw)
			}
		}
		t.Setenv(name, "")
	}
	t.Setenv("UPSTREAM_CONNECT_TIMEOUT", "5s")
	t.Setenv("UPSTREAM_TLS_TIMEOUT", "12s")
	t.Setenv("UPSTREAM_HEADER_TIMEOUT", "2m")
	transport, err := upstreamTransport()
	if err != nil || transport.TLSHandshakeTimeout != 12*time.Second || transport.ResponseHeaderTimeout != 2*time.Minute {
		t.Fatal(transport, err)
	}
}
func TestUpstreamHeaderTimeout(t *testing.T) {
	t.Setenv("UPSTREAM_HEADER_TIMEOUT", "20ms")
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
			return
		case <-time.After(time.Second):
			w.Write([]byte("late"))
		}
	}))
	defer upstream.Close()
	handler, err := proxyHandler(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "http://proxy/", nil))
	if response.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", response.Code)
	}
}
