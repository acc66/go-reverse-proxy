package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/netip"
	"strings"
)

type clientIPKey struct{}

func trustedProxyRanges(raw string) ([]netip.Prefix, error) {
	var result []netip.Prefix
	for _, value := range strings.Split(raw, ",") {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(value)
		if err != nil {
			addr, addrErr := netip.ParseAddr(value)
			if addrErr != nil {
				return nil, fmt.Errorf("invalid TRUSTED_PROXIES address or CIDR %q", value)
			}
			addr = addr.Unmap()
			prefix = netip.PrefixFrom(addr, addr.BitLen())
		}
		result = append(result, prefix.Masked())
	}
	return result, nil
}
func peerAddress(r *http.Request) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	addr, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	return addr.Unmap()
}
func addressText(addr netip.Addr) string {
	if !addr.IsValid() {
		return "unknown"
	}
	return addr.String()
}
func resolveClientIP(r *http.Request, trusted []netip.Prefix) string {
	peer := peerAddress(r)
	contains := func(addr netip.Addr) bool {
		for _, prefix := range trusted {
			if prefix.Contains(addr) {
				return true
			}
		}
		return false
	}
	if !peer.IsValid() || !contains(peer) {
		return addressText(peer)
	}
	raw := strings.Join(r.Header.Values("X-Forwarded-For"), ",")
	if strings.TrimSpace(raw) == "" {
		return addressText(peer)
	}
	// Invalid chains fall back to the transport peer; never log arbitrary header text.
	var chain []netip.Addr
	for _, part := range strings.Split(raw, ",") {
		addr, err := netip.ParseAddr(strings.TrimSpace(part))
		if err != nil || addr.Zone() != "" {
			return addressText(peer)
		}
		chain = append(chain, addr.Unmap())
	}
	current := peer
	for i := len(chain) - 1; i >= 0; i-- {
		if !contains(current) {
			break
		}
		current = chain[i]
	}
	return addressText(current)
}
func clientIP(r *http.Request) string {
	if value, ok := r.Context().Value(clientIPKey{}).(string); ok {
		return value
	}
	return addressText(peerAddress(r))
}
func withClientIP(r *http.Request, trusted []netip.Prefix) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), clientIPKey{}, resolveClientIP(r, trusted)))
}
