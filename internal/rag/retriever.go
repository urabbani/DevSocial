package rag

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"
)

// Client talks to ChromaDB for vector storage and retrieval.
type Client struct {
	baseURL string
	client  *http.Client
}

// NewClient creates a ChromaDB client.
func NewClient() *Client {
	baseURL := os.Getenv("CHROMA_URL")
	if baseURL == "" {
		baseURL = "http://localhost:8000"
	}
	return &Client{
		baseURL: baseURL,
		client:  &http.Client{Timeout: 30 * time.Second},
	}
}

// Health checks if ChromaDB is reachable.
func (c *Client) Health() error {
	resp, err := c.client.Get(c.baseURL + "/api/v1/heartbeat")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chromadb health check failed: %d", resp.StatusCode)
	}
	return nil
}

// EnsureCollection creates a collection if it doesn't exist.
func (c *Client) EnsureCollection(name string) error {
	body := map[string]any{
		"name": name,
	}
	data, _ := json.Marshal(body)
	resp, err := c.client.Post(c.baseURL+"/api/v1/collections", "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("create collection: %w", err)
	}
	defer resp.Body.Close()
	// 200 = created, 409 = already exists — both fine
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 409 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("create collection %d: %s", resp.StatusCode, string(respBody))
	}
	log.Printf("[rag] collection ensured: %s", name)
	return nil
}

// Document represents a document to be indexed.
type Document struct {
	ID        string
	Content   string
	Embedding []float32
	Metadata  map[string]string
}

// AddDocuments adds documents to a collection.
func (c *Client) AddDocuments(collectionName string, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	ids := make([]string, len(docs))
	embeddings := make([][]float32, len(docs))
	documents := make([]string, len(docs))
	metadatas := make([]map[string]string, len(docs))

	for i, d := range docs {
		ids[i] = d.ID
		embeddings[i] = d.Embedding
		documents[i] = d.Content
		metadatas[i] = d.Metadata
	}

	body := map[string]any{
		"ids":        ids,
		"embeddings": embeddings,
		"documents":  documents,
		"metadatas":  metadatas,
	}
	data, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v1/collections/%s/add", c.baseURL, collectionName)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("add documents: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 201 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("add documents %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// QueryResult is a single result from a similarity query.
type QueryResult struct {
	ID       string
	Content  string
	Metadata map[string]string
	Score    float64
}

// Query searches for similar documents in a collection.
func (c *Client) Query(collectionName string, embedding []float32, nResults int, where map[string]any) ([]QueryResult, error) {
	body := map[string]any{
		"query_embeddings": []any{embedding},
		"n_results":        nResults,
	}
	if where != nil {
		body["where"] = where
	}
	data, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v1/collections/%s/query", c.baseURL, collectionName)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("query: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("query %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Documents [][]string            `json:"documents"`
		IDs       [][]string            `json:"ids"`
		Metadatas [][]map[string]string `json:"metadatas"`
		Distances [][]float64           `json:"distances"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("parse query response: %w", err)
	}
	if len(result.Documents) == 0 || len(result.Documents[0]) == 0 {
		return nil, nil
	}

	results := make([]QueryResult, len(result.Documents[0]))
	for i := range result.Documents[0] {
		results[i] = QueryResult{
			ID:       result.IDs[0][i],
			Content:  result.Documents[0][i],
			Metadata: result.Metadatas[0][i],
			Score:    result.Distances[0][i],
		}
	}
	return results, nil
}

// DeleteDocuments removes documents from a collection by IDs.
func (c *Client) DeleteDocuments(collectionName string, ids []string) error {
	body := map[string]any{"ids": ids}
	data, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/api/v1/collections/%s/delete", c.baseURL, collectionName)
	resp, err := c.client.Post(url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("delete documents: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != 200 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("delete documents %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}
