package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestProxy(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if r.URL.Path != "/base/items" || r.URL.RawQuery != "a=1&b=2" || r.Method != "POST" || string(body) != "hello" {
			t.Errorf("incorrect forwarded request: %v %s", r.URL, body)
		}
		if strings.Contains(r.Header.Get("X-Forwarded-For"), "forged") {
			t.Error("trusted forged header")
		}
		w.Header().Set("X-Upstream", "yes")
		w.WriteHeader(201)
		w.Write([]byte("ok"))
	}))
	defer upstream.Close()
	handler, err := proxyHandler(upstream.URL + "/base?a=1")
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("POST", "http://proxy/items?b=2", strings.NewReader("hello"))
	req.Header.Set("X-Forwarded-For", "forged")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	if out.Code != 201 || out.Body.String() != "ok" || out.Header().Get("X-Upstream") != "yes" {
		t.Fatalf("unexpected response: %v", out)
	}
}
func TestInvalidUpstream(t *testing.T) {
	for _, raw := range []string{"", "ftp://example.com", "http://", "http://user:pass@example.com", "https://example.com/#x"} {
		if _, err := proxyHandler(raw); err == nil {
			t.Errorf("accepted %q", raw)
		}
	}
}
func TestBadGateway(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	upstream.Close()
	handler, _ := proxyHandler(upstream.URL)
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("GET", "http://proxy/", nil))
	if out.Code != 502 {
		t.Fatalf("got %d", out.Code)
	}
}
