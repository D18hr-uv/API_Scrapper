package embedder

import (
	"context"
	"fmt"

	"github.com/sashabaranov/go-openai"
)

var client *openai.Client

func Init() error {
	config := openai.DefaultConfig("skip-key")
	config.BaseURL = "http://localhost:11434/v1"
	client = openai.NewClientWithConfig(config)
	return nil
}

func GenerateEmbeddings(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	req := openai.EmbeddingRequest{
		Input: texts,
		Model: "nomic-embed-text", // local ollama 768 dimensions
	}

	req.Input = texts

	resp, err := client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error creating embeddings: %v", err)
	}

	var results [][]float32
	for _, data := range resp.Data {
		results = append(results, data.Embedding)
	}

	return results, nil
}
