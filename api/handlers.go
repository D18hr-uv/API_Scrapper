package api

import (
	"context"

	"api-scrapper/crawler"
	"api-scrapper/db"
	"api-scrapper/internal/embedder"

	"github.com/gofiber/fiber/v2"
	"github.com/pgvector/pgvector-go"
)

type SearchResponse struct {
	URL     string  `json:"url"`
	Title   string  `json:"title"`
	Content string  `json:"content"`
	Score   float32 `json:"score"`
}

// StartCrawlHandler kicks off a new web scrape
func StartCrawlHandler(c *fiber.Ctx) error {
	var job crawler.CrawlJob
	if err := c.BodyParser(&job); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request payload"})
	}

	if job.URL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "URL is required"})
	}
	if job.MaxDepth == 0 {
		job.MaxDepth = 1 // default
	}

	// Trigger async to prevent blocking the API
	go func() {
		_ = crawler.StartCrawl(context.Background(), job)
	}()

	return c.JSON(fiber.Map{"message": "Crawl job started successfully", "url": job.URL})
}

// SearchHandler performs a semantic search utilizing vector embeddings
func SearchHandler(c *fiber.Ctx) error {
	query := c.Query("q")
	if query == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Query parameter 'q' is required"})
	}

	ctx := context.Background()
	
	// Create embedding for the query (Nomic requires 'search_query: ' prefix for high accuracy)
	queryEmbs, err := embedder.GenerateEmbeddings(ctx, []string{"search_query: " + query})
	if err != nil || len(queryEmbs) == 0 {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to generate embedding for query"})
	}
	
	queryVector := pgvector.NewVector(queryEmbs[0])

	// HNSW Vector distance search via pgvector (Cosine similarity distance <->)
	// We map the chunk back to its parent page.
	rows, err := db.Pool.Query(ctx, `
		SELECT p.url, p.title, c.content, 1 - (c.embedding <=> $1) as similarity_score
		FROM chunks c
		JOIN pages p ON c.page_id = p.id
		ORDER BY c.embedding <=> $1 ASC
		LIMIT 10
	`, queryVector)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Database search failed", "details": err.Error()})
	}
	defer rows.Close()

	var results []SearchResponse
	for rows.Next() {
		var res SearchResponse
		if err := rows.Scan(&res.URL, &res.Title, &res.Content, &res.Score); err != nil {
			continue
		}
		results = append(results, res)
	}

	return c.JSON(fiber.Map{"query": query, "results": results})
}

// GraphHandler returns pages derived directly from a specific parent URL (navigational flow)
func GraphHandler(c *fiber.Ctx) error {
	parentURL := c.Query("parent_url")
	if parentURL == "" {
		return c.Status(400).JSON(fiber.Map{"error": "Query parameter 'parent_url' is required"})
	}

	rows, err := db.Pool.Query(context.Background(), `
		SELECT url, title FROM pages WHERE parent_url = $1
	`, parentURL)

	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve graph data"})
	}
	defer rows.Close()

	type PageNode struct {
		URL   string `json:"url"`
		Title string `json:"title"`
	}

	var results []PageNode
	for rows.Next() {
		var p PageNode
		if err := rows.Scan(&p.URL, &p.Title); err != nil {
			continue
		}
		results = append(results, p)
	}

	return c.JSON(fiber.Map{"parent_url": parentURL, "children": results})
}
