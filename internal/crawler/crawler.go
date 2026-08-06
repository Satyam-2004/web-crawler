package crawler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Satyam-2004/web-crawler/internal/models"
	"github.com/Satyam-2004/web-crawler/internal/storage"
	"golang.org/x/net/html"
	"golang.org/x/time/rate"
)

type Config struct {
	SeedURL       string
	MaxPages      int
	MaxDepth      int
	Workers       int
	RateLimit     rate.Limit
	UserAgent     string
	Timeout       time.Duration
	ContentLimit  int
	SameHost      bool
	RespectRobots bool
	OutputJSON    string
}

func DefaultConfig(seed string) Config {
	return Config{
		SeedURL:       seed,
		MaxPages:      100,
		MaxDepth:      2,
		Workers:       6,
		RateLimit:     rate.Limit(2),
		UserAgent:     "GoCrawler/1.0 (+https://github.com/Satyam-2004/web-crawler)",
		Timeout:       10 * time.Second,
		ContentLimit:  1000,
		SameHost:      true,
		RespectRobots: true,
		OutputJSON:    "",
	}
}

type job struct {
	url   string
	depth int
}

type Crawler struct {
	cfg       Config
	store     *storage.MongoStore
	visited   sync.Map
	queue     chan job
	wg        sync.WaitGroup
	client    *http.Client
	limiter   *rate.Limiter
	robots    *robotsCache
	seedHost  string
	pages     int64
	errors    int64
	blocked   int64
	start     time.Time
	results   []models.Page
	resultsMu sync.Mutex
}

func New(cfg Config, store *storage.MongoStore) *Crawler {
	u, _ := url.Parse(cfg.SeedURL)
	host := ""
	if u != nil {
		host = u.Host
	}
	return &Crawler{
		cfg:      cfg,
		store:    store,
		queue:    make(chan job, cfg.MaxPages*2),
		robots:   newRobotsCache(),
		seedHost: host,
		client: &http.Client{
			Timeout: cfg.Timeout,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		limiter: rate.NewLimiter(cfg.RateLimit, 1),
		results: make([]models.Page, 0, cfg.MaxPages),
	}
}

func (c *Crawler) Run() {
	ctx, cancel := context.WithCancel(context.Background())
	c.start = time.Now()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		fmt.Println("\nShutting down gracefully...")
		cancel()
	}()

	fmt.Printf("Starting crawl from %s\n", c.cfg.SeedURL)
	fmt.Printf("  max pages=%d  depth=%d  workers=%d  same-host=%v  robots=%v\n\n",
		c.cfg.MaxPages, c.cfg.MaxDepth, c.cfg.Workers, c.cfg.SameHost, c.cfg.RespectRobots)

	c.queue <- job{url: c.cfg.SeedURL, depth: 0}
	c.visited.Store(normalize(c.cfg.SeedURL), true)

	for i := 0; i < c.cfg.Workers; i++ {
		c.wg.Add(1)
		go c.worker(ctx)
	}

	ticker := time.NewTicker(300 * time.Millisecond)
	defer ticker.Stop()

loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case <-ticker.C:
			if atomic.LoadInt64(&c.pages) >= int64(c.cfg.MaxPages) {
				cancel()
				break loop
			}
		}
	}

	close(c.queue)
	c.wg.Wait()
	c.printSummary()
	c.exportJSON()
}

func (c *Crawler) worker(ctx context.Context) {
	defer c.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case j, ok := <-c.queue:
			if !ok {
				return
			}
			if atomic.LoadInt64(&c.pages) >= int64(c.cfg.MaxPages) {
				return
			}
			c.crawl(ctx, j)
		}
	}
}

func (c *Crawler) crawl(ctx context.Context, j job) {
	if c.cfg.RespectRobots && !c.robots.allowed(j.url, c.cfg.UserAgent) {
		atomic.AddInt64(&c.blocked, 1)
		return
	}

	if err := c.limiter.Wait(ctx); err != nil {
		return
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, j.url, nil)
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return
	}
	req.Header.Set("User-Agent", c.cfg.UserAgent)

	resp, err := c.client.Do(req)
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 512*1024))
	if err != nil {
		atomic.AddInt64(&c.errors, 1)
		return
	}

	title, content, links := extract(body, j.url, c.cfg.ContentLimit)

	page := models.Page{
		URL:       j.url,
		Title:     title,
		Content:   content,
		Status:    resp.StatusCode,
		Depth:     j.depth,
		CrawledAt: time.Now().UTC(),
	}

	if c.store != nil {
		_ = c.store.Save(page)
	}

	c.resultsMu.Lock()
	c.results = append(c.results, page)
	c.resultsMu.Unlock()

	n := atomic.AddInt64(&c.pages, 1)
	fmt.Printf("[%4d] d=%d  %s\n      → %s\n", n, j.depth, j.url, truncate(title, 70))

	if j.depth >= c.cfg.MaxDepth || atomic.LoadInt64(&c.pages) >= int64(c.cfg.MaxPages) {
		return
	}

	for _, link := range links {
		norm := normalize(link)
		if norm == "" {
			continue
		}
		if c.cfg.SameHost {
			u, err := url.Parse(norm)
			if err != nil || u.Host != c.seedHost {
				continue
			}
		}
		if _, loaded := c.visited.LoadOrStore(norm, true); !loaded {
			select {
			case c.queue <- job{url: norm, depth: j.depth + 1}:
			default:
			}
		}
	}
}

func (c *Crawler) printSummary() {
	elapsed := time.Since(c.start).Round(time.Millisecond)
	fmt.Println("\n==================== CRAWL SUMMARY ====================")
	fmt.Printf("Pages crawled     : %d\n", atomic.LoadInt64(&c.pages))
	fmt.Printf("Errors            : %d\n", atomic.LoadInt64(&c.errors))
	fmt.Printf("Blocked (robots)  : %d\n", atomic.LoadInt64(&c.blocked))
	fmt.Printf("Duration          : %s\n", elapsed)
	if atomic.LoadInt64(&c.pages) > 0 {
		fmt.Printf("Avg time per page : %s\n", (elapsed / time.Duration(atomic.LoadInt64(&c.pages))).Round(time.Millisecond))
	}
	if c.store != nil && c.store.Enabled() {
		if n, err := c.store.Count(); err == nil {
			fmt.Printf("Stored in MongoDB : %d\n", n)
		}
	}
	fmt.Println("=======================================================")
}

func (c *Crawler) exportJSON() {
	if c.cfg.OutputJSON == "" {
		return
	}
	c.resultsMu.Lock()
	defer c.resultsMu.Unlock()

	f, err := os.Create(c.cfg.OutputJSON)
	if err != nil {
		fmt.Printf("Failed to write JSON: %v\n", err)
		return
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c.results); err != nil {
		fmt.Printf("Failed to encode JSON: %v\n", err)
		return
	}
	fmt.Printf("Results exported to %s (%d pages)\n", c.cfg.OutputJSON, len(c.results))
}

func extract(body []byte, baseURL string, contentLimit int) (title, content string, links []string) {
	base, _ := url.Parse(baseURL)
	z := html.NewTokenizer(bytes.NewReader(body))
	inBody := false
	var contentBuilder strings.Builder

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			break
		}
		t := z.Token()

		switch {
		case t.Type == html.StartTagToken && t.Data == "title":
			if z.Next() == html.TextToken {
				title = strings.TrimSpace(z.Token().Data)
			}
		case t.Type == html.StartTagToken && t.Data == "body":
			inBody = true
		case t.Type == html.StartTagToken && t.Data == "a":
			for _, a := range t.Attr {
				if a.Key == "href" {
					resolved := resolveURL(base, a.Val)
					if resolved != "" {
						links = append(links, resolved)
					}
				}
			}
		case inBody && t.Type == html.TextToken && contentBuilder.Len() < contentLimit:
			text := strings.TrimSpace(t.Data)
			if text != "" {
				contentBuilder.WriteString(text)
				contentBuilder.WriteByte(' ')
			}
		}
	}
	return title, strings.TrimSpace(contentBuilder.String()), links
}

func resolveURL(base *url.URL, ref string) string {
	ref = strings.TrimSpace(ref)
	if ref == "" || strings.HasPrefix(ref, "#") || strings.HasPrefix(ref, "mailto:") || strings.HasPrefix(ref, "javascript:") {
		return ""
	}
	u, err := url.Parse(ref)
	if err != nil {
		return ""
	}
	if base != nil {
		u = base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	return u.String()
}

func normalize(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Fragment = ""
	if u.Path == "" {
		u.Path = "/"
	}
	return u.String()
}

func truncate(s string, n int) string {
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}
