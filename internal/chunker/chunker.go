package chunker

import (
	"strings"
)

func ChunkText(text string, maxWordsPerChunk int) []string {
	words := strings.Fields(text)
	var chunks []string

	var currentChunk []string

	for _, word := range words {
		currentChunk = append(currentChunk, word)

		if len(currentChunk) >= maxWordsPerChunk {
			chunks = append(chunks, strings.Join(currentChunk, " "))
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
