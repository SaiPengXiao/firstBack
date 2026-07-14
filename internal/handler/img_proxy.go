package handler

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	imgProxyMaxBodySize   = 5 << 20 // 5MB，防止恶意大文件耗尽内存
	imgProxyFetchTimeout  = 10 * time.Second
	imgProxyCacheTTL      = 10 * time.Minute
	imgProxyMaxCacheItems = 512 // 粗粒度上限，避免缓存无限膨胀
	imgProxyMaxRedirects  = 3
)

// 预计算的禁止网段（Go 标准库未完整覆盖的部分）
var (
	cgnatNet = mustCIDR("100.64.0.0/10") // RFC 6598 Carrier-Grade NAT
	ipv6ULA  = mustCIDR("fc00::/7")      // Unique Local Address
)

func mustCIDR(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}

// imgCacheEntry 内存缓存条目。
type imgCacheEntry struct {
	data        []byte
	contentType string
	expiresAt   time.Time
}

// imgMemoryCache 带 TTL 的简单内存缓存（读多写少场景足够）。
type imgMemoryCache struct {
	mu      sync.RWMutex
	entries map[string]imgCacheEntry
}

func newImgMemoryCache() *imgMemoryCache {
	c := &imgMemoryCache{entries: make(map[string]imgCacheEntry)}
	go c.janitor()
	return c
}

func (c *imgMemoryCache) janitor() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		c.mu.Lock()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		c.mu.Unlock()
	}
}

func (c *imgMemoryCache) get(key string) (imgCacheEntry, bool) {
	c.mu.RLock()
	e, ok := c.entries[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.expiresAt) {
		if ok {
			c.mu.Lock()
			delete(c.entries, key)
			c.mu.Unlock()
		}
		return imgCacheEntry{}, false
	}
	return e, true
}

func (c *imgMemoryCache) set(key string, data []byte, contentType string, ttl time.Duration) {
	// 拷贝一份，避免持有上游 slice 引用
	copied := make([]byte, len(data))
	copy(copied, data)

	c.mu.Lock()
	defer c.mu.Unlock()
	// 超限时做一次过期清理；仍超限则丢弃最旧策略的简化版：清空后写入
	if len(c.entries) >= imgProxyMaxCacheItems {
		now := time.Now()
		for k, e := range c.entries {
			if now.After(e.expiresAt) {
				delete(c.entries, k)
			}
		}
		if len(c.entries) >= imgProxyMaxCacheItems {
			c.entries = make(map[string]imgCacheEntry)
		}
	}
	c.entries[key] = imgCacheEntry{
		data:        copied,
		contentType: contentType,
		expiresAt:   time.Now().Add(ttl),
	}
}

// ImgProxyHandler 统一图片代理网关。
type ImgProxyHandler struct {
	client *http.Client
	cache  *imgMemoryCache
}

// NewImgProxyHandler 创建带 SSRF 防护 Dialer 的代理处理器。
func NewImgProxyHandler() *ImgProxyHandler {
	dialer := &net.Dialer{
		Timeout:   5 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		Proxy: nil, // 禁止走环境代理，避免意外把流量导向内网代理
		// DialContext 层强制按已解析 IP 拨号并再次校验，防御 DNS Rebinding
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			host, port, err := net.SplitHostPort(addr)
			if err != nil {
				return nil, fmt.Errorf("split addr: %w", err)
			}
			if err := assertSafePort(port); err != nil {
				return nil, err
			}

			// 若 host 本身就是 IP，直接校验
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
				target := net.JoinHostPort(ipa.IP.String(), port)
				conn, err := dialer.DialContext(ctx, network, target)
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
		ResponseHeaderTimeout: imgProxyFetchTimeout,
		ForceAttemptHTTP2:     true,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   imgProxyFetchTimeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= imgProxyMaxRedirects {
				return errors.New("too many redirects")
			}
			// 每次跳转重新做 URL / Host SSRF 校验（Dial 还会再拦一遍）
			if err := validateProxyTargetURL(req.URL); err != nil {
				return fmt.Errorf("redirect blocked: %w", err)
			}
			return nil
		},
	}

	return &ImgProxyHandler{
		client: client,
		cache:  newImgMemoryCache(),
	}
}

// Proxy GET /img-proxy?url=<encodeURIComponent(原始图片URL)>
func (h *ImgProxyHandler) Proxy(c *gin.Context) {
	raw := c.Query("url")
	if raw == "" {
		writeImgProxyError(c, http.StatusBadRequest, "missing query parameter: url")
		return
	}

	// Gin / net/http 已对 query 做一次解码；再尝试一次以兼容双重编码场景
	decoded, err := url.QueryUnescape(raw)
	if err != nil {
		writeImgProxyError(c, http.StatusBadRequest, "invalid url encoding")
		return
	}
	if decoded == "" {
		writeImgProxyError(c, http.StatusBadRequest, "empty url")
		return
	}

	target, err := url.Parse(decoded)
	if err != nil {
		writeImgProxyError(c, http.StatusBadRequest, "invalid url")
		return
	}
	if err := validateProxyTargetURL(target); err != nil {
		writeImgProxyError(c, http.StatusForbidden, err.Error())
		return
	}

	cacheKey := target.String()
	if entry, ok := h.cache.get(cacheKey); ok {
		c.Header("Cache-Control", "public, max-age=86400")
		c.Header("X-Cache", "HIT")
		c.Data(http.StatusOK, entry.contentType, entry.data)
		return
	}

	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, target.String(), nil)
	if err != nil {
		writeImgProxyError(c, http.StatusInternalServerError, "failed to create upstream request")
		return
	}
	req.Header.Set("User-Agent", "firstgo-back-img-proxy/1.0")
	req.Header.Set("Accept", "image/*,*/*;q=0.8")

	resp, err := h.client.Do(req)
	if err != nil {
		writeImgProxyError(c, http.StatusBadGateway, "upstream fetch failed: "+sanitizeErr(err))
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		writeImgProxyError(c, http.StatusBadGateway, fmt.Sprintf("upstream status %d", resp.StatusCode))
		return
	}

	ct := resp.Header.Get("Content-Type")
	mediaType := strings.TrimSpace(strings.Split(ct, ";")[0])
	mediaType = strings.ToLower(mediaType)
	if !strings.HasPrefix(mediaType, "image/") {
		writeImgProxyError(c, http.StatusUnsupportedMediaType, "upstream content-type is not image/*")
		return
	}

	// 多读 1 字节以检测是否越界
	limited := io.LimitReader(resp.Body, imgProxyMaxBodySize+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		writeImgProxyError(c, http.StatusBadGateway, "failed to read upstream body")
		return
	}
	if len(data) > imgProxyMaxBodySize {
		writeImgProxyError(c, http.StatusRequestEntityTooLarge, "image exceeds 5MB limit")
		return
	}

	h.cache.set(cacheKey, data, mediaType, imgProxyCacheTTL)

	c.Header("Cache-Control", "public, max-age=86400")
	c.Header("X-Cache", "MISS")
	c.Data(http.StatusOK, mediaType, data)
}

// validateProxyTargetURL 请求前静态校验：协议、凭证、主机、端口、字面 IP。
func validateProxyTargetURL(u *url.URL) error {
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
	lowerHost := strings.ToLower(host)
	if lowerHost == "localhost" || strings.HasSuffix(lowerHost, ".localhost") ||
		lowerHost == "metadata.google.internal" {
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

	// 字面量 IP 提前拒绝内网地址
	if ip := net.ParseIP(host); ip != nil {
		if isForbiddenIP(ip) {
			return fmt.Errorf("target IP not allowed: %s", ip)
		}
		return nil
	}

	// 请求前解析一次，尽早失败；真正连接时 DialContext 还会再校验（防 rebinding）
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
	// 图片代理仅允许标准 Web 端口，缩小 SSRF 攻击面
	if port != "80" && port != "443" {
		return fmt.Errorf("port not allowed: %s", port)
	}
	return nil
}

// isForbiddenIP 判断是否为回环 / 私有 / 链路本地等不可达公网目标。
func isForbiddenIP(ip net.IP) bool {
	if ip == nil {
		return true
	}
	// To4 会把 IPv4-mapped IPv6（::ffff:x.x.x.x）归一成 4 字节，避免重复分支递归
	if v4 := ip.To4(); v4 != nil {
		ip = v4
	}

	if ip.IsLoopback() || // 127.0.0.0/8, ::1
		ip.IsPrivate() || // 10/8, 172.16/12, 192.168/16（及较新 Go 的 ULA）
		ip.IsLinkLocalUnicast() || // 169.254.0.0/16, fe80::/10
		ip.IsLinkLocalMulticast() ||
		ip.IsMulticast() ||
		ip.IsUnspecified() || // 0.0.0.0, ::
		ip.IsInterfaceLocalMulticast() {
		return true
	}

	if cgnatNet.Contains(ip) {
		return true
	}
	// 仅对纯 IPv6 再判 ULA，双保险
	if ip.To4() == nil && ipv6ULA.Contains(ip) {
		return true
	}
	return false
}

func writeImgProxyError(c *gin.Context, status int, msg string) {
	c.Header("Content-Type", "application/json; charset=utf-8")
	c.Status(status)
	_ = json.NewEncoder(c.Writer).Encode(gin.H{
		"error": msg,
	})
}

func sanitizeErr(err error) string {
	// 避免把完整 dial 细节 / 内网 IP 拼进响应（仍保留可读原因）
	msg := err.Error()
	if len(msg) > 200 {
		return msg[:200]
	}
	return msg
}
