package models

import "time"

// Page represents a crawled webpage.
type Page struct {
	URL       string    `bson:"url" json:"url"`
	Title     string    `bson:"title" json:"title"`
	Content   string    `bson:"content,omitempty" json:"content,omitempty"`
	Status    int       `bson:"status" json:"status"`
	Depth     int       `bson:"depth" json:"depth"`
	CrawledAt time.Time `bson:"crawled_at" json:"crawled_at"`
}
