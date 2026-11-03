package main

import (
	"bytes"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAccessLogRedaction(t *testing.T) {
	t.Setenv("ACCESS_LOG", "true")
	var output bytes.Buffer
	previous := log.Writer()
	log.SetOutput(&output)
	defer log.SetOutput(previous)
	handler, err := accessLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(201)
		w.Write([]byte("ok"))
	}))
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("GET", "http://proxy/secret-path?token=private-token", nil)
	req.Header.Set("Authorization", "Bearer secret-auth")
	req.Header.Set("Cookie", "secret-cookie")
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, req)
	for _, secret := range []string{"secret-path", "private-token", "secret-auth", "secret-cookie"} {
		if strings.Contains(output.String(), secret) {
			t.Fatal("sensitive log")
		}
	}
	if !strings.Contains(output.String(), "status=201") || !strings.Contains(output.String(), "bytes=2") {
		t.Fatal(output.String())
	}
	t.Setenv("ACCESS_LOG", "invalid")
	if _, err := accessLogging(http.NotFoundHandler()); err == nil {
		t.Fatal("invalid value accepted")
	}
}
func TestLoggingPreservesFlush(t *testing.T) {
	t.Setenv("ACCESS_LOG", "true")
	handler, _ := accessLogging(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := http.NewResponseController(w).Flush(); err != nil {
			t.Error(err)
		}
	}))
	out := httptest.NewRecorder()
	handler.ServeHTTP(out, httptest.NewRequest("GET", "http://proxy/", nil))
	if !out.Flushed {
		t.Fatal("flush lost")
	}
}
func TestEncodingVariants(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "public, max-age=60")
		w.Header().Set("Vary", "Accept-Encoding")
		w.Write([]byte(r.Header.Get("Accept-Encoding")))
	}))
	defer upstream.Close()
	handler, _ := proxyHandler(upstream.URL)
	for _, encoding := range []string{"identity", "br"} {
		for i := 0; i < 2; i++ {
			req := httptest.NewRequest("GET", "http://proxy/app.js", nil)
			req.Header.Set("Accept-Encoding", encoding)
			out := httptest.NewRecorder()
			handler.ServeHTTP(out, req)
			if out.Body.String() != encoding {
				t.Fatal("variant mixed")
			}
			if i == 1 && out.Header().Get("X-Proxy-Cache") != "HIT" {
				t.Fatal("variant not cached")
			}
		}
	}
	for _, vary := range []string{"*", "Cookie", "Accept-Encoding, Origin", "Authorization"} {
		if cacheableVary(http.Header{"Vary": []string{vary}}) {
			t.Fatalf("accepted Vary %s", vary)
		}
	}
}
