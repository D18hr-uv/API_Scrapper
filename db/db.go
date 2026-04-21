package db

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/jackc/pgx/v5/pgxpool"
)

var Pool *pgxpool.Pool

func InitDB() error {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return fmt.Errorf("DATABASE_URL environment variable is not set")
	}

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return fmt.Errorf("unable to parse DATABASE_URL: %v", err)
	}

	Pool, err = pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		return fmt.Errorf("unable to connect to database: %v", err)
	}

	log.Println("Connected to PostgreSQL successfully")

	err = createTables(context.Background())
	if err != nil {
		return fmt.Errorf("could not create tables: %v", err)
	}

	return nil
}

func createTables(ctx context.Context) error {
	_, _ = Pool.Exec(ctx, "DROP TABLE IF EXISTS chunks, pages CASCADE")

	// Ensure the pgvector extension is enabled
	_, err := Pool.Exec(ctx, "CREATE EXTENSION IF NOT EXISTS vector")
	if err != nil {
		return fmt.Errorf("error creating vector extension: %v", err)
	}

	// Create Pages table
	_, err = Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS pages (
			id SERIAL PRIMARY KEY,
			url TEXT UNIQUE NOT NULL,
			parent_url TEXT,
			title TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating pages table: %v", err)
	}

	// Create Chunks table
	_, err = Pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS chunks (
			id SERIAL PRIMARY KEY,
			page_id INT REFERENCES pages(id) ON DELETE CASCADE,
			content TEXT NOT NULL,
			embedding VECTOR(768)
		)
	`)
	if err != nil {
		return fmt.Errorf("error creating chunks table: %v", err)
	}

	// Create index for fast vector searching
	_, err = Pool.Exec(ctx, `
		CREATE INDEX IF NOT EXISTS chunks_embedding_idx ON chunks USING hnsw (embedding vector_cosine_ops);
	`)
	if err != nil {
		log.Printf("Warning: Could not create pgvector index (might already exist): %v", err)
	}

	log.Println("Database schema initialized successfully")
	return nil
}

func CloseDB() {
	if Pool != nil {
		Pool.Close()
	}
}
