package crawler

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"strings"
	"time"

	"api-scrapper/db"
	"api-scrapper/internal/chunker"
	"api-scrapper/internal/embedder"

	"github.com/gocolly/colly/v2"
	"github.com/pgvector/pgvector-go"
)

type CrawlJob struct {
	URL       string `json:"url"`
	MaxDepth  int    `json:"max_depth"`
}

// StartCrawl triggers the crawling process for a given URL
func StartCrawl(ctx context.Context, job CrawlJob) error {
	log.Printf("Starting crawl for: %s with depth %d", job.URL, job.MaxDepth)

	parsedURL, err := url.Parse(job.URL)
	if err != nil {
		return fmt.Errorf("invalid url: %v", err)
	}
	domain := parsedURL.Host

	c := colly.NewCollector(
		colly.AllowedDomains(domain),
		colly.MaxDepth(job.MaxDepth),
		colly.Async(true),
	)

	// Rate limiting to avoid bans
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 2,
		Delay:       1 * time.Second,
	})

	c.OnHTML("html", func(e *colly.HTMLElement) {
		pageURL := e.Request.URL.String()
		title := e.ChildText("title")

		// Get all text from paragraphs
		var contentBuilder strings.Builder
		e.ForEach("p, h1, h2, h3, h4, h5, h6, li", func(_ int, el *colly.HTMLElement) {
			text := strings.TrimSpace(el.Text)
			if len(text) > 20 { // filter short useless text
				contentBuilder.WriteString(text)
				contentBuilder.WriteString("\n")
			}
		})

		content := contentBuilder.String()
		if len(content) < 50 {
			return // Skip pages with too little meaningful content
		}

		// Insert page to get ID
		var pageID int
		parentURL := ""
		if e.Request.Ctx.Get("parent_url") != "" {
			parentURL = e.Request.Ctx.Get("parent_url")
		}

		err := db.Pool.QueryRow(ctx, `
			INSERT INTO pages (url, parent_url, title) 
			VALUES ($1, $2, $3)
			ON CONFLICT (url) DO UPDATE SET title = EXCLUDED.title 
			RETURNING id
		`, pageURL, parentURL, title).Scan(&pageID)

		if err != nil {
			log.Printf("Error inserting page %s: %v", pageURL, err)
			return
		}

		// Process content into chunks
		chunks := chunker.ChunkText(content, 200) // approx 200 words per chunk

		if len(chunks) > 0 {
			// Generate embeddings in batches to save API calls
			embeddings, err := embedder.GenerateEmbeddings(ctx, chunks)
			if err != nil {
				log.Printf("Error generating embeddings for %s: %v", pageURL, err)
				return
			}

			// Store chunks and their embeddings
			for i, chunkText := range chunks {
				if i >= len(embeddings) {
					break
				}
				emb := pgvector.NewVector(embeddings[i])

				_, err := db.Pool.Exec(ctx, `
					INSERT INTO chunks (page_id, content, embedding)
					VALUES ($1, $2, $3)
				`, pageID, chunkText, emb)

				if err != nil {
					log.Printf("Error inserting chunk: %v", err)
				}
			}
			log.Printf("Processed %s -> %d chunks stored", pageURL, len(chunks))
		}
	})

	c.OnHTML("a[href]", func(e *colly.HTMLElement) {
		link := e.Attr("href")
		absoluteURL := e.Request.AbsoluteURL(link)

		if absoluteURL != "" && strings.HasPrefix(absoluteURL, "http") {
			// Inform context of parent URL for the next request
			e.Request.Ctx.Put("parent_url", e.Request.URL.String())
			e.Request.Visit(absoluteURL)
		}
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("Request URL: %s failed with error: %v", r.Request.URL, err)
	})

	// Start Crawling
	c.Visit(job.URL)
	c.Wait()

	log.Printf("Crawl finished for: %s", job.URL)
	return nil
}
