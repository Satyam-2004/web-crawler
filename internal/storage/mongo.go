package storage

import (
	"context"
	"fmt"
	"time"

	"github.com/Satyam-2004/web-crawler/internal/models"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type MongoStore struct {
	client     *mongo.Client
	collection *mongo.Collection
	enabled    bool
}

func NewMongoStore(uri string) (*MongoStore, error) {
	if uri == "" {
		return &MongoStore{enabled: false}, nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %w", err)
	}

	if err := client.Ping(ctx, nil); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	col := client.Database("webcrawler").Collection("pages")
	return &MongoStore{
		client:     client,
		collection: col,
		enabled:    true,
	}, nil
}

func (s *MongoStore) Save(page models.Page) error {
	if !s.enabled {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_, err := s.collection.InsertOne(ctx, page)
	return err
}

func (s *MongoStore) Count() (int64, error) {
	if !s.enabled {
		return 0, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return s.collection.CountDocuments(ctx, bson.D{})
}

func (s *MongoStore) Close() {
	if s.enabled && s.client != nil {
		_ = s.client.Disconnect(context.Background())
	}
}

func (s *MongoStore) Enabled() bool {
	return s.enabled
}
