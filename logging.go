package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

type accessWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *accessWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }
func (w *accessWriter) WriteHeader(code int) {
	if code >= 100 && code < 200 && code != 101 {
		w.ResponseWriter.WriteHeader(code)
		return
	}
	if w.status != 0 {
		return
	}
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}
func (w *accessWriter) Write(p []byte) (int, error) {
	if w.status == 0 {
		w.WriteHeader(http.StatusOK)
	}
	n, err := w.ResponseWriter.Write(p)
	w.bytes += n
	return n, err
}
func accessLogging(handler http.Handler) (http.Handler, error) {
	trusted, err := trustedProxyRanges(os.Getenv("TRUSTED_PROXIES"))
	if err != nil {
		return nil, err
	}
	raw := strings.TrimSpace(os.Getenv("ACCESS_LOG"))
	enabled := false
	if raw != "" {
		enabled, err = strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("ACCESS_LOG must be true or false")
		}
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r = withClientIP(r, trusted)
		if !enabled {
			handler.ServeHTTP(w, r)
			return
		}
		start := time.Now()
		out := &accessWriter{ResponseWriter: w}
		defer func() {
			status := out.status
			if status == 0 {
				status = http.StatusOK
			}
			log.Printf("access ip=%s method=%s status=%d duration_ms=%d bytes=%d cache=%s", clientIP(r), r.Method, status, time.Since(start).Milliseconds(), out.bytes, out.Header().Get("X-Proxy-Cache"))
		}()
		handler.ServeHTTP(out, r)
	}), nil
}
