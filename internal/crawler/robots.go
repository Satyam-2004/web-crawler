package crawler

import (
	"bufio"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// robotsCache stores parsed robots.txt rules per host.
type robotsCache struct {
	mu    sync.RWMutex
	rules map[string]*robotRules
}

type robotRules struct {
	disallow []string
	fetched  bool
}

func newRobotsCache() *robotsCache {
	return &robotsCache{rules: make(map[string]*robotRules)}
}

func (rc *robotsCache) allowed(rawURL, userAgent string) bool {
	u, err := url.Parse(rawURL)
	if err != nil {
		return false
	}
	host := u.Host

	rc.mu.RLock()
	rules, ok := rc.rules[host]
	rc.mu.RUnlock()

	if !ok {
		rules = rc.fetch(host, userAgent)
	}

	path := u.Path
	if path == "" {
		path = "/"
	}

	for _, d := range rules.disallow {
		if d == "" {
			continue
		}
		if strings.HasPrefix(path, d) {
			return false
		}
	}
	return true
}

func (rc *robotsCache) fetch(host, userAgent string) *robotRules {
	rc.mu.Lock()
	defer rc.mu.Unlock()

	// Double-check
	if rules, ok := rc.rules[host]; ok {
		return rules
	}

	rules := &robotRules{fetched: true}
	robotsURL := fmt.Sprintf("https://%s/robots.txt", host)

	client := &http.Client{Timeout: 5 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, robotsURL, nil)
	req.Header.Set("User-Agent", userAgent)

	resp, err := client.Do(req)
	if err != nil || resp.StatusCode != 200 {
		rc.rules[host] = rules
		return rules
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	applicable := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "user-agent:") {
			ua := strings.TrimSpace(line[11:])
			applicable = ua == "*" || strings.Contains(strings.ToLower(userAgent), strings.ToLower(ua))
			continue
		}
		if applicable && strings.HasPrefix(lower, "disallow:") {
			path := strings.TrimSpace(line[9:])
			if path != "" {
				rules.disallow = append(rules.disallow, path)
			}
		}
	}

	rc.rules[host] = rules
	return rules
}
