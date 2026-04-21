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

		// We split the parsed HTML into sections so that URLs can directly point to #anchors!
		type Section struct {
			Anchor      string
			TextBuilder strings.Builder
		}
		
		var sections []Section
		currentSection := Section{Anchor: ""}

		e.ForEach("p, h1, h2, h3, h4, h5, h6, li", func(_ int, el *colly.HTMLElement) {
			name := el.Name
			// If it's a header with an ID, finish the last section and start a new one
			if (name == "h1" || name == "h2" || name == "h3" || name == "h4" || name == "h5" || name == "h6") && el.Attr("id") != "" {
				if currentSection.TextBuilder.Len() > 0 {
					sections = append(sections, currentSection)
				}
				currentSection = Section{Anchor: "#" + el.Attr("id")}
			}

			text := strings.TrimSpace(el.Text)
			if len(text) > 20 { // filter short useless text
				currentSection.TextBuilder.WriteString(text)
				currentSection.TextBuilder.WriteString("\n")
			}
		})

		// Append the last section
		if currentSection.TextBuilder.Len() > 0 {
			sections = append(sections, currentSection)
		}

		parentURL := ""
		if e.Request.Ctx.Get("parent_url") != "" {
			parentURL = e.Request.Ctx.Get("parent_url")
		}

		// Save each section as a separate "page" in the DB
		for _, sec := range sections {
			content := sec.TextBuilder.String()
			if len(content) < 50 {
				continue // Skip small sections
			}

			secURL := pageURL
			currentParentURL := parentURL
			secTitle := title

			if sec.Anchor != "" {
				secURL = pageURL + sec.Anchor
				currentParentURL = pageURL // Anchor nodes are children of the root page url
				secTitle = title + " (" + sec.Anchor + ")"
			}

			var pageID int
			err := db.Pool.QueryRow(ctx, `
				INSERT INTO pages (url, parent_url, title) 
				VALUES ($1, $2, $3)
				ON CONFLICT (url) DO UPDATE SET title = EXCLUDED.title 
				RETURNING id
			`, secURL, currentParentURL, secTitle).Scan(&pageID)

			if err != nil {
				log.Printf("Error inserting page %s: %v", secURL, err)
				continue
			}

			// Clear old chunks so we don't duplicate on re-crawls
			_, _ = db.Pool.Exec(ctx, "DELETE FROM chunks WHERE page_id = $1", pageID)

			// Process content into chunks
			chunks := chunker.ChunkText(content, 200)

			// Nomic-embed-text requires a specific prefix for high accuracy on documents
			var embeddingTexts []string
			for _, chunkText := range chunks {
				embeddingTexts = append(embeddingTexts, "search_document: "+chunkText)
			}

			if len(chunks) > 0 {
				embeddings, err := embedder.GenerateEmbeddings(ctx, embeddingTexts)
				if err != nil {
					log.Printf("Error generating embeddings for %s: %v", secURL, err)
					continue
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
				log.Printf("Processed %s -> %d chunks stored", secURL, len(chunks))
			}
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
