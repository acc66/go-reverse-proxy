package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func proxyHandler(raw string) (http.Handler, error) {
	target, err := url.Parse(raw)
	if err != nil || target.Hostname() == "" || (target.Scheme != "http" && target.Scheme != "https") || target.User != nil || target.Fragment != "" {
		return nil, errors.New("UPSTREAM must be an http(s) URL without credentials or fragment")
	}
	excludedPaths, err := parseExcludedPaths(os.Getenv("CACHE_BYPASS_PATHS"))
	if err != nil {
		return nil, err
	}
	transport, err := upstreamTransport()
	if err != nil {
		return nil, err
	}
	cache := newAssetCache(transport)
	cache.excludedPaths = excludedPaths
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
			r.Out.Host = target.Host
			r.SetXForwarded()
		},
		Transport: cache,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("upstream request failed ip=%s method=%s upstream=%s error=%v", clientIP(r), r.Method, target.Host, err)
			http.Error(w, "Bad Gateway", http.StatusBadGateway)
		},
	}
	return accessLogging(proxy)
}

func main() {
	handler, err := proxyHandler(strings.TrimSpace(os.Getenv("UPSTREAM")))
	if err != nil {
		log.Fatal(err)
	}
	addr := os.Getenv("LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	server := &http.Server{Addr: addr, Handler: handler, ReadHeaderTimeout: 10 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 1 << 20}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			server.Close()
		}
	}()
	log.Printf("listening on %s", addr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal(err)
	}
}
