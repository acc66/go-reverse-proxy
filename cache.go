package main

import (
	"bytes"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"
)

type cachedAsset struct {
	header http.Header
	body   []byte
	stored time.Time
	ttl    time.Duration
}
type assetCache struct {
	base      http.RoundTripper
	mu        sync.Mutex
	entries   map[string]cachedAsset
	used      int
	limit     int
	maxObject int
}

func newAssetCache(base http.RoundTripper) *assetCache {
	return &assetCache{base: base, entries: make(map[string]cachedAsset), limit: 64 << 20, maxObject: 2 << 20}
}
func staticRequest(r *http.Request) bool {
	if r.Method != "GET" || r.Header.Get("Cookie") != "" || r.Header.Get("Authorization") != "" || r.Header.Get("Range") != "" || r.Header.Get("Cache-Control") != "" || r.Header.Get("Pragma") != "" || r.Header.Get("Upgrade") != "" {
		return false
	}
	for k := range r.Header {
		if strings.HasPrefix(strings.ToLower(k), "if-") {
			return false
		}
	}
	p := strings.ToLower(path.Clean("/" + r.URL.Path))
	for _, prefix := range []string{"/api", "/admin", "/user", "/wp-admin", "/wp-login.php"} {
		if p == prefix || strings.HasPrefix(p, prefix+"/") {
			return false
		}
	}
	switch strings.ToLower(path.Ext(p)) {
	case ".css", ".js", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".avif", ".ico", ".woff", ".woff2", ".ttf":
		return true
	}
	return false
}

// Only explicitly public responses with a finite freshness lifetime are shared.
func assetTTL(h http.Header) time.Duration {
	if len(h.Values("Set-Cookie")) > 0 || h.Get("Vary") != "" || h.Get("Content-Range") != "" {
		return 0
	}
	public := false
	seconds := int64(-1)
	for _, token := range strings.Split(strings.ToLower(strings.Join(h.Values("Cache-Control"), ",")), ",") {
		pair := strings.SplitN(strings.TrimSpace(token), "=", 2)
		switch pair[0] {
		case "private", "no-store", "no-cache":
			return 0
		case "public":
			public = true
		case "s-maxage", "max-age":
			if len(pair) != 2 {
				return 0
			}
			n, err := strconv.ParseInt(strings.Trim(pair[1], "\""), 10, 64)
			if err != nil || n <= 0 {
				return 0
			}
			if seconds < 0 || n < seconds {
				seconds = n
			}
		}
	}
	if !public || seconds <= 0 {
		return 0
	}
	if seconds > 3600 {
		seconds = 3600
	}
	age := int64(0)
	if raw := h.Get("Age"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return 0
		}
		age = n
	}
	if age >= seconds {
		return 0
	}
	return time.Duration(seconds-age) * time.Second
}

type joinedBody struct {
	io.Reader
	io.Closer
}

func (c *assetCache) RoundTrip(r *http.Request) (*http.Response, error) {
	eligible := staticRequest(r)
	key := r.URL.String() + "\n" + r.Host + "\n" + r.Header.Get("Accept-Encoding")
	if eligible {
		c.mu.Lock()
		item, ok := c.entries[key]
		if ok && time.Since(item.stored) >= item.ttl {
			delete(c.entries, key)
			c.used -= assetSize(key, item)
			ok = false
		}
		c.mu.Unlock()
		if ok {
			h := item.header.Clone()
			originalAge, _ := strconv.ParseInt(h.Get("Age"), 10, 64)
			h.Set("Age", strconv.FormatInt(originalAge+int64(time.Since(item.stored)/time.Second), 10))
			h.Set("X-Proxy-Cache", "HIT")
			return &http.Response{StatusCode: 200, Status: "200 OK", Header: h, Body: io.NopCloser(bytes.NewReader(item.body)), ContentLength: int64(len(item.body)), Request: r}, nil
		}
	}
	response, err := c.base.RoundTrip(r)
	if err != nil {
		return response, err
	}
	response.Header.Set("X-Proxy-Cache", "BYPASS")
	if !eligible || response.StatusCode != 200 {
		return response, nil
	}
	response.Header.Set("X-Proxy-Cache", "MISS")
	ttl := assetTTL(response.Header)
	if ttl <= 0 || response.ContentLength > int64(c.maxObject) || len(response.Trailer) > 0 {
		return response, nil
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, int64(c.maxObject)+1))
	if readErr != nil {
		response.Body.Close()
		return nil, readErr
	}
	if len(body) > c.maxObject {
		response.Body = joinedBody{io.MultiReader(bytes.NewReader(body), response.Body), response.Body}
		return response, nil
	}
	response.Body.Close()
	response.Body = io.NopCloser(bytes.NewReader(body))
	item := cachedAsset{header: response.Header.Clone(), body: body, stored: time.Now(), ttl: ttl}
	size := assetSize(key, item)
	c.mu.Lock()
	defer c.mu.Unlock()
	if size > c.limit {
		return response, nil
	}
	if old, ok := c.entries[key]; ok {
		c.used -= assetSize(key, old)
		delete(c.entries, key)
	}
	for k, v := range c.entries {
		if time.Since(v.stored) >= v.ttl {
			delete(c.entries, k)
			c.used -= assetSize(k, v)
		}
	}
	for c.used+size > c.limit {
		var oldest string
		var stamp time.Time
		for k, v := range c.entries {
			if stamp.IsZero() || v.stored.Before(stamp) {
				oldest = k
				stamp = v.stored
			}
		}
		c.used -= assetSize(oldest, c.entries[oldest])
		delete(c.entries, oldest)
	}
	c.entries[key] = item
	c.used += size
	return response, nil
}
func assetSize(key string, item cachedAsset) int {
	size := len(key) + len(item.body) + 256
	for k, values := range item.header {
		size += len(k)
		for _, v := range values {
			size += len(v) + 32
		}
	}
	return size
}
