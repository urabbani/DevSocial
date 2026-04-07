package app

import (
	"math"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type rateRule struct {
	Name  string
	Rate  float64
	Burst float64
}

type rateBucket struct {
	Tokens     float64
	LastRefill time.Time
	LastSeen   time.Time
}

type IPRateLimiter struct {
	mu          sync.Mutex
	buckets     map[string]*rateBucket
	lastCleanup time.Time
}

func NewIPRateLimiter() *IPRateLimiter {
	return &IPRateLimiter{
		buckets:     make(map[string]*rateBucket),
		lastCleanup: time.Now(),
	}
}

func (rl *IPRateLimiter) Allow(key string, rule rateRule, now time.Time) (bool, int) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.cleanup(now)

	bucket, ok := rl.buckets[key]
	if !ok {
		bucket = &rateBucket{
			Tokens:     rule.Burst,
			LastRefill: now,
			LastSeen:   now,
		}
		rl.buckets[key] = bucket
	} else {
		elapsed := now.Sub(bucket.LastRefill).Seconds()
		bucket.Tokens = minFloat(rule.Burst, bucket.Tokens+elapsed*rule.Rate)
		bucket.LastRefill = now
		bucket.LastSeen = now
	}

	if bucket.Tokens >= 1 {
		bucket.Tokens -= 1
		return true, 0
	}

	if rule.Rate <= 0 {
		return false, 60
	}
	retryAfter := int(math.Ceil((1 - bucket.Tokens) / rule.Rate))
	if retryAfter < 1 {
		retryAfter = 1
	}
	return false, retryAfter
}

func (rl *IPRateLimiter) cleanup(now time.Time) {
	if now.Sub(rl.lastCleanup) < 5*time.Minute {
		return
	}
	for key, bucket := range rl.buckets {
		if now.Sub(bucket.LastSeen) > 30*time.Minute {
			delete(rl.buckets, key)
		}
	}
	rl.lastCleanup = now
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func rateRuleForRequest(r *http.Request) (rateRule, bool) {
	path := r.URL.Path

	if strings.HasPrefix(path, "/static/") || strings.HasPrefix(path, "/uploads/") {
		return rateRule{}, false
	}

	switch {
	case path == "/login" || strings.HasPrefix(path, "/auth/"):
		return rateRule{Name: "auth", Rate: 0.5, Burst: 20}, true
	case strings.HasPrefix(path, "/api/") || path == "/docs.md" || strings.HasSuffix(path, "/feed.xml"):
		return rateRule{Name: "read", Rate: 2, Burst: 60}, true
	case r.Method == http.MethodPost:
		return rateRule{Name: "write", Rate: 1, Burst: 20}, true
	default:
		return rateRule{Name: "page", Rate: 4, Burst: 120}, true
	}
}

func clientIP(r *http.Request) string {
	remoteIP := parseIPFromAddr(r.RemoteAddr)
	if remoteIP != nil && trustForwardedHeaders(remoteIP) {
		if ip := parseForwardedIP(r.Header.Get("X-Forwarded-For")); ip != nil {
			return ip.String()
		}
		if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
			return ip.String()
		}
	}
	if remoteIP != nil {
		return remoteIP.String()
	}

	if ip := parseForwardedIP(r.Header.Get("X-Forwarded-For")); ip != nil {
		return ip.String()
	}
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-IP"))); ip != nil {
		return ip.String()
	}
	return "unknown"
}

func parseIPFromAddr(addr string) net.IP {
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
	}
	return net.ParseIP(strings.TrimSpace(host))
}

func parseForwardedIP(value string) net.IP {
	if value == "" {
		return nil
	}
	for _, part := range strings.Split(value, ",") {
		if ip := net.ParseIP(strings.TrimSpace(part)); ip != nil {
			return ip
		}
	}
	return nil
}

func trustForwardedHeaders(ip net.IP) bool {
	return ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast()
}
