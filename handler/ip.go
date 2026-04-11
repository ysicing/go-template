package handler

import (
	"net"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v3"
)

var (
	trustedCIDRs []*net.IPNet
	trustedMu    sync.RWMutex
)

// SetTrustedProxies configures the CIDR ranges from which forwarded headers are trusted.
// Must be called at startup before serving requests.
func SetTrustedProxies(cidrs []string) {
	var parsed []*net.IPNet
	for _, cidr := range cidrs {
		cidr = strings.TrimSpace(cidr)
		if cidr == "" {
			continue
		}
		// If it's a bare IP, wrap it as /32 or /128
		if !strings.Contains(cidr, "/") {
			if strings.Contains(cidr, ":") {
				cidr += "/128"
			} else {
				cidr += "/32"
			}
		}
		_, network, err := net.ParseCIDR(cidr)
		if err == nil {
			parsed = append(parsed, network)
		}
	}
	trustedMu.Lock()
	trustedCIDRs = parsed
	trustedMu.Unlock()
}

// isTrustedProxy checks if the given IP is within any trusted proxy CIDR range.
func isTrustedProxy(ip string) bool {
	// Strip port if present
	host, _, err := net.SplitHostPort(ip)
	if err != nil {
		host = ip
	}
	parsed := net.ParseIP(host)
	if parsed == nil {
		return false
	}
	trustedMu.RLock()
	defer trustedMu.RUnlock()
	for _, cidr := range trustedCIDRs {
		if cidr.Contains(parsed) {
			return true
		}
	}
	return false
}

// GetRealIP extracts the real client IP from the request, handling multiple proxy layers.
// Forwarded headers (CF-Connecting-IP, X-Real-IP, X-Forwarded-For) are only trusted
// when the direct connection comes from a configured trusted proxy.
func GetRealIP(c fiber.Ctx) string {
	remoteAddr := c.RequestCtx().RemoteAddr().String()

	if !isTrustedProxy(remoteAddr) {
		// Direct connection from untrusted source — use RemoteAddr only
		host, _, err := net.SplitHostPort(remoteAddr)
		if err != nil {
			return remoteAddr
		}
		return host
	}

	// Connection from trusted proxy — read forwarded headers
	// 1. CF-Connecting-IP (Cloudflare)
	if ip := c.Get("CF-Connecting-IP"); ip != "" {
		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			return ip
		}
	}

	// 2. X-Real-IP (Nginx/Caddy)
	if ip := c.Get("X-Real-IP"); ip != "" {
		if parsedIP := net.ParseIP(ip); parsedIP != nil {
			return ip
		}
	}

	// 3. X-Forwarded-For (take leftmost IP)
	if xff := c.Get("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if parsedIP := net.ParseIP(clientIP); parsedIP != nil {
				return clientIP
			}
		}
	}

	// 4. Fallback
	return c.IP()
}

// GetRealIPForRateLimit returns a rate limit key based on real client IP.
// This is a convenience wrapper for rate limiting use cases.
func GetRealIPForRateLimit(c fiber.Ctx, prefix string) string {
	return prefix + GetRealIP(c)
}

// GetRealIPAndUA is a convenience function that returns both real IP and User-Agent.
// This is commonly used for audit logging and session tracking.
func GetRealIPAndUA(c fiber.Ctx) (ip, ua string) {
	return GetRealIP(c), c.Get("User-Agent")
}
