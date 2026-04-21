package chunker

import (
	"strings"
)

// ChunkText splits a large text into smaller chunks based on max length.
// For MVP, we split by sentences or simple word count to avoid complexity.
func ChunkText(text string, maxWordsPerChunk int) []string {
	words := strings.Fields(text)
	var chunks []string
	
	var currentChunk []string
	
	for _, word := range words {
		currentChunk = append(currentChunk, word)
		
		if len(currentChunk) >= maxWordsPerChunk {
			chunks = append(chunks, strings.Join(currentChunk, " "))
			// Keep a small overlap context (e.g., last 10 words) for semantic continuation
			overlapSize := 10
			if len(currentChunk) > overlapSize {
				currentChunk = currentChunk[len(currentChunk)-overlapSize:]
			} else {
				currentChunk = nil
			}
		}
	}
	
	if len(currentChunk) > 0 {
		chunks = append(chunks, strings.Join(currentChunk, " "))
	}
	
	return chunks
}
