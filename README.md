# Concurrent Web Crawler in Go

A production-style, polite concurrent web crawler written in Go.

**Worker pools · Rate limiting · robots.txt · Depth control · JSON export · Graceful shutdown · Optional MongoDB**

![Go](https://img.shields.io/badge/Go-1.21+-00ADD8)

---

## Features

| Feature | Description |
|---------|-------------|
| **Worker Pool** | Fixed number of goroutines (no unbounded spawning) |
| **Rate Limiting** | Polite crawling using `golang.org/x/time/rate` |
| **robots.txt** | Fetches and respects Disallow rules per host |
| **Same-host filter** | Optionally stay within the seed domain |
| **Depth & page limits** | Bounded, predictable crawls |
| **URL normalization** | Resolves relative links, strips fragments, deduplicates |
| **Graceful shutdown** | Handles Ctrl+C cleanly |
| **JSON export** | Save results even without a database |
| **MongoDB** | Optional persistence |
| **CLI flags** | Fully configurable from the command line |

---

## Quick Start

```bash
# Basic run (no MongoDB needed)
go run ./cmd/crawler -seed https://books.toscrape.com/ -max 30 -depth 2

# Export results to JSON
go run ./cmd/crawler -seed https://books.toscrape.com/ -max 40 -out results.json

# Allow all hosts + ignore robots (use carefully)
go run ./cmd/crawler -seed https://example.com -same-host=false -robots=false -max 20
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

## Project Structure

```
web-crawler/
├── cmd/crawler/main.go           # CLI entrypoint
├── internal/
│   ├── crawler/
│   │   ├── crawler.go            # Core engine
│   │   └── robots.go             # robots.txt support
│   ├── models/page.go
│   └── storage/mongo.go
├── go.mod
└── README.md
```

---

## Design Highlights

- Worker pool instead of spawning a goroutine per URL
- Token-bucket rate limiter for politeness
- Per-host robots.txt cache
- Relative URL resolution against the page base
- Context cancellation for clean Ctrl+C shutdown
- Optional storage – works fully without MongoDB

---

## Author

**Satyam Pravinkumar Sharma**  
[GitHub](https://github.com/Satyam-2004)
