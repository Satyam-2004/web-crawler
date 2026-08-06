package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/Satyam-2004/web-crawler/internal/crawler"
	"github.com/Satyam-2004/web-crawler/internal/storage"
	"github.com/joho/godotenv"
)

func main() {
	seed := flag.String("seed", "https://books.toscrape.com/", "Seed URL to start crawling")
	maxPages := flag.Int("max", 50, "Maximum number of pages to crawl")
	depth := flag.Int("depth", 2, "Maximum crawl depth")
	workers := flag.Int("workers", 6, "Number of concurrent workers")
	sameHost := flag.Bool("same-host", true, "Only crawl pages on the same host as seed")
	robots := flag.Bool("robots", true, "Respect robots.txt")
	out := flag.String("out", "", "Export results to JSON file (e.g. results.json)")
	flag.Parse()

	_ = godotenv.Load()

	store, err := storage.NewMongoStore(os.Getenv("MONGODB_URI"))
	if err != nil {
		fmt.Printf("Warning: MongoDB unavailable (%v). Continuing without persistence.\n", err)
		store, _ = storage.NewMongoStore("")
	}
	defer store.Close()

	if store.Enabled() {
		fmt.Println("MongoDB connected – pages will be stored.")
	}

	cfg := crawler.DefaultConfig(*seed)
	cfg.MaxPages = *maxPages
	cfg.MaxDepth = *depth
	cfg.Workers = *workers
	cfg.SameHost = *sameHost
	cfg.RespectRobots = *robots
	cfg.OutputJSON = *out

	c := crawler.New(cfg, store)
	c.Run()
}
