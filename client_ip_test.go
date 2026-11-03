package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClientIPTrust(t *testing.T) {
	trusted, err := trustedProxyRanges("127.0.0.1,::1/128,10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ peer, xff, want string }{
		{"203.0.113.4:123", "1.2.3.4", "203.0.113.4"},
		{"127.0.0.1:123", "198.51.100.9", "198.51.100.9"},
		{"127.0.0.1:123", "1.2.3.4, 198.51.100.9", "198.51.100.9"},
		{"127.0.0.1:123", "198.51.100.9, 10.1.2.3", "198.51.100.9"},
		{"[::1]:123", "2001:db8::2", "2001:db8::2"},
		{"[::ffff:127.0.0.1]:123", "198.51.100.9", "198.51.100.9"},
		{"127.0.0.1:123", "not-an-ip", "127.0.0.1"},
		{"127.0.0.1:123", "", "127.0.0.1"},
	} {
		r := httptest.NewRequest("GET", "http://proxy/", nil)
		r.RemoteAddr = tc.peer
		r.Header.Set("X-Forwarded-For", tc.xff)
		if got := resolveClientIP(r, trusted); got != tc.want {
			t.Fatalf("%+v got %s", tc, got)
		}
	}
	if _, err := trustedProxyRanges("bad"); err == nil {
		t.Fatal("invalid proxy accepted")
	}
	r := httptest.NewRequest("GET", "http://proxy/", nil)
	r.RemoteAddr = "127.0.0.1:123"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	if got := resolveClientIP(r, nil); got != "127.0.0.1" {
		t.Fatal(got)
	}
}
func TestErrorLogClientIPWithAccessLogDisabled(t *testing.T) {
	t.Setenv("ACCESS_LOG", "false")
	t.Setenv("TRUSTED_PROXIES", "127.0.0.1/32")
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	server.Close()
	handler, err := proxyHandler(server.URL)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest("GET", "http://proxy/", nil)
	r.RemoteAddr = "127.0.0.1:123"
	r.Header.Set("X-Forwarded-For", "198.51.100.9")
	handler.ServeHTTP(httptest.NewRecorder(), r)
	if !strings.Contains(output.String(), "upstream request failed ip=198.51.100.9") {
		t.Fatal(output.String())
	}
	if strings.Contains(output.String(), "access ip=") {
		t.Fatal("unexpected access log")
	}
}
