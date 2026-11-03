package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAssetCache(t *testing.T) {
	hits := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.Header().Set("Cache-Control", "public, max-age=60")
		if r.URL.Query().Has("private") {
			w.Header().Set("Cache-Control", "private, max-age=60")
		}
		if r.URL.Query().Has("cookie") {
			w.Header().Set("Set-Cookie", "session=x")
		}
		if r.URL.Query().Has("vary") {
			w.Header().Set("Vary", "Accept-Language")
		}
		w.Write([]byte("asset"))
	}))
	defer upstream.Close()
	handler, _ := proxyHandler(upstream.URL)
	request := func(method, url string, headers map[string]string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, "http://proxy"+url, nil)
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, req)
		return out
	}
	for _, tc := range []struct {
		url, method string
		headers     map[string]string
	}{
		{"/app.js", "GET", nil},
		{"/app.js?v=2", "GET", nil},
	} {
		before := hits
		request(tc.method, tc.url, tc.headers)
		out := request(tc.method, tc.url, tc.headers)
		if hits != before+1 || out.Header().Get("X-Proxy-Cache") != "HIT" || out.Body.String() != "asset" {
			t.Fatal("cache miss", hits, out)
		}
	}
	for _, tc := range []struct {
		url, method string
		headers     map[string]string
	}{
		{"/api/login.js", "GET", nil}, {"/app.js", "POST", nil},
		{"/app.js", "GET", map[string]string{"Cookie": "session=x"}},
		{"/app.js", "GET", map[string]string{"Authorization": "Bearer x"}},
		{"/app.js", "GET", map[string]string{"Range": "bytes=0-2"}},
		{"/app.js", "GET", map[string]string{"If-None-Match": "x"}},
		{"/app.js?private", "GET", nil}, {"/app.js?cookie", "GET", nil},
		{"/app.js?vary", "GET", nil}, {"/index.html", "GET", nil},
	} {
		before := hits
		request(tc.method, tc.url, tc.headers)
		request(tc.method, tc.url, tc.headers)
		if hits != before+2 {
			t.Fatalf("unsafe caching: %+v", tc)
		}
	}
}
func TestTTL(t *testing.T) {
	for _, value := range []string{"", "max-age=60", "public, no-store, max-age=60", "public, max-age=0", "private, public, max-age=60"} {
		h := http.Header{"Cache-Control": []string{value}}
		if assetTTL(h) != 0 {
			t.Fatal(value)
		}
	}
	h := http.Header{"Cache-Control": []string{"public, max-age=60"}, "Age": []string{"59"}}
	if assetTTL(h) != time.Second {
		t.Fatal("age not accounted for")
	}
}
func TestCacheBoundsAndExpiry(t *testing.T) {
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte(strings.Repeat("a", 16)))
	}))
	defer upstream.Close()
	c := newAssetCache(http.DefaultTransport)
	get := func(p string) {
		req, _ := http.NewRequest("GET", upstream.URL+p, nil)
		res, err := c.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		res.Body.Close()
	}
	c.maxObject = 8
	get("/large.js")
	get("/large.js")
	if calls != 2 || len(c.entries) != 0 {
		t.Fatal("large object cached")
	}
	c.maxObject = 32
	c.limit = 600
	get("/one.js")
	get("/two.js")
	if c.used > c.limit {
		t.Fatal("budget exceeded")
	}
	for k, v := range c.entries {
		v.stored = time.Now().Add(-time.Hour)
		c.entries[k] = v
	}
	before := calls
	get("/two.js")
	if calls != before+1 {
		t.Fatal("expired entry served")
	}
}

func TestCustomBypassPaths(t *testing.T) {
	paths, err := parseExcludedPaths(" /Private/*, /auth/, /Private, , /single.js ")
	if err != nil || len(paths) != 3 {
		t.Fatalf("parse: %v %v", paths, err)
	}
	for _, p := range []string{"/private/file.js", "/PRIVATE/file.js", "/auth/a.css", "/single.js", "/api/file.js"} {
		if staticRequest(httptest.NewRequest("GET", "http://proxy"+p, nil), paths...) {
			t.Fatalf("not bypassed: %s", p)
		}
	}
	if !staticRequest(httptest.NewRequest("GET", "http://proxy/private-other/file.js", nil), paths...) {
		t.Fatal("prefix boundary lost")
	}
	all, err := parseExcludedPaths("/*")
	if err != nil || staticRequest(httptest.NewRequest("GET", "http://proxy/app.js", nil), all...) {
		t.Fatal("root exclusion failed")
	}
	for _, raw := range []string{"https://example.com/a", "relative", "/a?x=1", "/a*b", "/a%2fb", "/a\nb"} {
		if _, err := parseExcludedPaths(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
}
func TestConfiguredBypassIntegration(t *testing.T) {
	t.Setenv("CACHE_BYPASS_PATHS", "/private")
	calls := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Write([]byte("asset"))
	}))
	defer upstream.Close()
	handler, err := proxyHandler(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		out := httptest.NewRecorder()
		handler.ServeHTTP(out, httptest.NewRequest("GET", "http://proxy/private/app.js", nil))
		if out.Header().Get("X-Proxy-Cache") != "BYPASS" {
			t.Fatal("custom bypass not used")
		}
	}
	if calls != 2 {
		t.Fatal("private asset cached")
	}
	t.Setenv("CACHE_BYPASS_PATHS", "invalid")
	if _, err := proxyHandler(upstream.URL); err == nil {
		t.Fatal("invalid configuration accepted")
	}
}
