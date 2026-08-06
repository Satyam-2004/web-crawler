# Concurrent Web Crawler in Go

A production-style, polite concurrent web crawler written in Go.

**Worker pools · Rate limiting · robots.txt · Depth control · JSON export · Graceful shutdown · Performance tracking · Optional MongoDB**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8)

---

## Overview

This crawler fetches web pages concurrently using a fixed worker pool, respects `robots.txt`, stays on the seed host by default, and records performance metrics over time. Results can be stored in MongoDB or exported to JSON.

It is designed to be **bounded, polite, and observable** — the qualities expected in real crawlers.

---

## How it works

```
┌─────────────┐     job queue      ┌──────────────┐
│  Seed URL   │ ───────────────►  │ Worker Pool  │
└─────────────┘                   │  (N goroutines)│
                                  └──────┬───────┘
                                         │
                    ┌────────────────────┼────────────────────┐
                    ▼                    ▼                    ▼
             Rate Limiter          robots.txt check      HTTP Fetch
                    │                    │                    │
                    └────────────────────┼────────────────────┘
                                         ▼
                                  HTML Parse
                                   (title, text, links)
                                         │
                    ┌────────────────────┼────────────────────┐
                    ▼                    ▼                    ▼
              URL Normalize        Visited Set           Enqueue new
              + Same-host          (sync.Map)            links (depth+1)
                    │
                    ▼
         MongoDB / JSON export + live stats
```

1. Seed URL is pushed into a buffered job queue  
2. Fixed worker goroutines pull jobs  
3. Each worker: rate-limit → robots check → fetch → parse → save → enqueue new links  
4. `sync.Map` guarantees no URL is visited twice  
5. Context cancellation handles Ctrl+C and max-page limit  
6. A background ticker records pages crawled every 5 seconds for performance analysis

---

## Features

| Feature | Description |
|---------|-------------|
| **Worker Pool** | Fixed number of goroutines (no unbounded spawning) |
| **Rate Limiting** | Token-bucket limiter (default 2 req/sec) |
| **robots.txt** | Per-host fetch + Disallow respect |
| **Same-host filter** | Stay on seed domain (default on) |
| **Depth & page limits** | Bounded, predictable crawls |
| **Relative URL resolution** | Handles `/path`, `../`, etc. correctly |
| **Graceful shutdown** | Ctrl+C stops cleanly |
| **Performance tracker** | Pages/sec over time printed at the end |
| **JSON export** | `-out results.json` |
| **MongoDB** | Optional persistence |
| **CLI flags** | Fully configurable |

---

## Quick Start

```bash
# Basic run
go run ./cmd/crawler -seed https://books.toscrape.com/ -max 30 -depth 2

# Export to JSON + see performance stats
go run ./cmd/crawler -seed https://books.toscrape.com/ -max 40 -out results.json

# Crawl more aggressively (use carefully)
go run ./cmd/crawler -seed https://example.com -same-host=false -robots=false -max 20 -workers 10
```

### With MongoDB

```env
# .env
MONGODB_URI=mongodb://localhost:27017
```

```bash
go run ./cmd/crawler -seed https://books.toscrape.com/ -max 50
```

---

## CLI Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-seed` | `https://books.toscrape.com/` | Starting URL |
| `-max` | `50` | Max pages to crawl |
| `-depth` | `2` | Max link depth |
| `-workers` | `6` | Concurrent workers |
| `-same-host` | `true` | Stay on seed host only |
| `-robots` | `true` | Respect robots.txt |
| `-out` | `""` | Write results to JSON file |

---

## Example Output

```
Starting crawl from https://books.toscrape.com/
  max pages=30  depth=2  workers=6  same-host=true  robots=true

[   1] d=0  https://books.toscrape.com/
      → All products | Books to Scrape - Sandbox
[   2] d=1  https://books.toscrape.com/catalogue/page-2.html
      → All products | Books to Scrape - Sandbox
...

==================== CRAWL SUMMARY ====================
Pages crawled     : 30
Errors            : 0
Blocked (robots)  : 0
Duration          : 16.2s
Avg time per page : 540ms
Crawl speed       : 1.85 pages/sec

Performance over time:
  Elapsed(s) | Pages | Speed (pages/s)
        5.0 |    9 | 1.80
       10.0 |   18 | 1.80
       15.0 |   28 | 2.00
=======================================================
Results exported to results.json (30 pages)
```

---

## Project Structure

```
web-crawler/
├── cmd/crawler/main.go           # CLI entrypoint
├── internal/
│   ├── crawler/
│   │   ├── crawler.go            # Core engine + performance tracking
│   │   └── robots.go             # robots.txt support
│   ├── models/page.go            # Page document
│   └── storage/mongo.go          # Optional MongoDB store
├── go.mod
└── README.md
```

---

## Design Decisions

**Pros**
- Controlled concurrency via worker pool (avoids resource exhaustion)
- Polite by default (rate limit + robots.txt + same-host)
- Observable (live progress + final performance table)
- Works with or without MongoDB
- Clean separation of concerns (`cmd` / `internal`)

**Trade-offs / Limitations**
- Simple robots.txt parser (supports common Disallow rules, not full RFC)
- Content extraction is text-only (first N characters of body)
- No sitemap.xml or priority queues yet
- Single-process (not distributed)

These limitations are intentional to keep the core clear and extensible.

---

## Possible Extensions

- Sitemap.xml support
- Swappable strategies (BFS / DFS / priority)
- Redis-backed distributed queue
- Prometheus metrics endpoint
- Retry with exponential backoff

---

## Author

**Satyam Pravinkumar Sharma**  
[GitHub](https://github.com/Satyam-2004) · [LinkedIn](https://linkedin.com/in/satyam-pravinkumar-sharma)
