// Package main 提供可独立运行的图片代理网关（纯标准库）。
//
// 运行：
//
//	go run ./cmd/imgproxy
//
// 示例：
//
//	curl "http://127.0.0.1:8088/img-proxy?url=$(python3 -c 'import urllib.parse;print(urllib.parse.quote("https://httpbin.org/image/png",safe=""))')"
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"
)

const (
	maxBodySize   = 5 << 20 // 5MB
	fetchTimeout  = 10 * time.Second
	cacheTTL      = 10 * time.Minute
	maxCacheItems = 512
	maxRedirects  = 3
	listenDefault = ":8088"
)

var (
	cgnatNet = mustCIDR("100.64.0.0/10")
	ipv6ULA  = mustCIDR("fc00::/7")
)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

type cacheEntry struct {
	data        []byte
	contentType string
	expiresAt   time.Time
}

type memoryCache struct {
	mu      sync.RWMutex
	entries map[string]cacheEntry
}

func newMemoryCache() *memoryCache {
	c := &memoryCache{entries: make(map[string]cacheEntry)}
	go func() {
		t := time.NewTicker(time.Minute)
		defer t.Stop()
		for range t.C {
			now := time.Now()
			c.mu.Lock()
			for k, e := range c.entries {
				if now.After(e.expiresAt) {
					delete(c.entries, k)
				}
			}
			c.mu.Unlock()
		}
	}()
	return c
}

func (c *memoryCache) get(key string) (cacheEntry, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return cacheEntry{}, false
	}
	return e, true
}

func (c *memoryCache) set(key string, data []byte, contentType string, ttl time.Duration) {
	copied := make([]byte, len(data))
	copy(copied, data)
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) >= maxCacheItems {
		now := time.Now()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= maxCacheItems {
			c.entries = make(map[string]cacheEntry)
		}
	}
	c.entries[key] = cacheEntry{
		data:        copied,
		contentType: contentType,
		expiresAt:   time.Now().Add(ttl),
	}
}

type proxyServer struct {
	client *http.Client
	cache  *memoryCache
}

func newProxyServer() *proxyServer {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: nil,
		// Dial 层按解析后的 IP 拨号并二次校验，防御 DNS Rebinding
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("split addr: %w", err)
			}
			if err := assertSafePort(port); err != nil {
				return nil, err
			}
			if ip := net.ParseIP(host); ip != nil {
				if isForbiddenIP(ip) {
					return nil, fmt.Errorf("forbidden IP: %s", ip)
				}
				return dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
			}
			ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
			if err != nil {
				return nil, fmt.Errorf("dns lookup: %w", err)
			}
			var lastErr error
			for _, ipa := range ips {
				if isForbiddenIP(ipa.IP) {
					lastErr = fmt.Errorf("forbidden resolved IP: %s", ipa.IP)
					continue
				}
				conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ipa.IP.String(), port))
				if err != nil {
					lastErr = err
					continue
				}
				return conn, nil
			}
			if lastErr == nil {
				lastErr = errors.New("no usable IP after SSRF filter")
			}
			return nil, lastErr
		},
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: fetchTimeout,
		ForceAttemptHTTP2:     true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   fetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return errors.New("too many redirects")
			}
			if err := validateTargetURL(req.URL); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}

	return &proxyServer{client: client, cache: newMemoryCache()}
}

func (s *proxyServer) handleImgProxy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	raw := r.URL.Query().Get("url")
	if raw == "" {
		writeJSONError(w, http.StatusBadRequest, "missing query parameter: url")
		return
	}
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid url encoding")
		return
	}
	if decoded == "" {
		writeJSONError(w, http.StatusBadRequest, "empty url")
		return
	}

	target, err := url.Parse(decoded)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid url")
		return
	}
	if err := validateTargetURL(target); err != nil {
		writeJSONError(w, http.StatusForbidden, err.Error())
		return
	}

	cacheKey := target.String()
	if entry, ok := s.cache.get(cacheKey); ok {
		w.Header().Set("Content-Type", entry.contentType)
		w.Header().Set("Cache-Control", "public, max-age=86400")
		w.Header().Set("X-Cache", "HIT")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(entry.data)
		return
	}

	req, err := http.NewRequestWithContext(r.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "failed to create upstream request")
		return
	}
	req.Header.Set("User-Agent", "img-proxy/1.0")
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	resp, err := s.client.Do(req)
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "upstream fetch failed: "+truncate(err.Error(), 200))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeJSONError(w, http.StatusBadGateway, fmt.Sprintf("upstream status %d", resp.StatusCode))
		return
	}

	ct := resp.Header.Get("Content-Type")
	mediaType := strings.ToLower(strings.TrimSpace(strings.Split(ct, ";")[0]))
	if !strings.HasPrefix(mediaType, "image/") {
		writeJSONError(w, http.StatusUnsupportedMediaType, "upstream content-type is not image/*")
		return
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize+1))
	if err != nil {
		writeJSONError(w, http.StatusBadGateway, "failed to read upstream body")
		return
	}
	if len(data) > maxBodySize {
		writeJSONError(w, http.StatusRequestEntityTooLarge, "image exceeds 5MB limit")
		return
	}

	s.cache.set(cacheKey, data, mediaType, cacheTTL)

	w.Header().Set("Content-Type", mediaType)
	w.Header().Set("Cache-Control", "public, max-age=86400")
	w.Header().Set("X-Cache", "MISS")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(data)
}

func validateTargetURL(u *url.URL) error {
	if u == nil {
		return errors.New("nil url")
	}
	scheme := strings.ToLower(u.Scheme)
	if scheme != "http" && scheme != "https" {
		return fmt.Errorf("protocol not allowed: %s", u.Scheme)
	}
	if u.User != nil {
		return errors.New("url must not contain userinfo")
	}
	host := u.Hostname()
	if host == "" {
		return errors.New("empty host")
	}
	lower := strings.ToLower(host)
	if lower == "localhost" || strings.HasSuffix(lower, ".localhost") ||
		lower == "metadata.google.internal" {
		return errors.New("hostname not allowed")
	}

	port := u.Port()
	if port == "" {
		if scheme == "https" {
			port = "443"
		} else {
			port = "80"
		}
	}
	if err := assertSafePort(port); err != nil {
		return err
	}

	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return fmt.Errorf("target IP not allowed: %s", ip)
		}
		return nil
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("dns resolve failed: %w", err)
	}
	if len(ips) == 0 {
		return errors.New("dns resolve returned no addresses")
	}
	for _, ip := range ips {
		if isForbiddenIP(ip) {
			return fmt.Errorf("resolved IP not allowed: %s", ip)
		}
	}
	return nil
}

func assertSafePort(port string) error {
	if port != "80" && port != "443" {
		return fmt.Errorf("port not allowed: %s", port)
	}
	return nil
}

func isForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// To4 会把 IPv4-mapped IPv6（::ffff:x.x.x.x）归一成 4 字节，避免重复分支递归
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}
	if ip.IsLoopback() ||
		ip.IsPrivate() ||
		ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() ||
		ip.IsInterfaceLocalMulticast() {
		return true
	}
	if cgnatNet.Contains(ip) {
		return true
	}
	// 仅对纯 IPv6 再判 ULA（IsPrivate 在较新 Go 已覆盖，这里保留双保险）
	if ip.To4() == nil && ipv6ULA.Contains(ip) {
		return true
	}
	return false
}

func writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func main() {
	addr := listenDefault
	if v := os.Getenv("IMG_PROXY_ADDR"); v != "" {
		addr = v
	}
	s := newProxyServer()
	mux := http.NewServeMux()
	mux.HandleFunc("/img-proxy", s.handleImgProxy)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	log.Printf("img-proxy listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}
