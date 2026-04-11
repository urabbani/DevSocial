package search

import (
	"context"
	"database/sql"
	"fmt"
	"log"

	"devsocial/internal/ai"
)

// SearchResult represents a single search result with relevance score.
type SearchResult struct {
	Type        string `json:"type"`
	ID          int64  `json:"id"`
	Title       string `json:"title"`
	Preview     string `json:"preview"`
	Author      string `json:"author,omitempty"`
	Date        string `json:"date"`
	WorkspaceID int64  `json:"workspace_id"`
	Score       float64 `json:"score"` // Similarity score (0-1, higher is better)
}

// SemanticSearcher handles semantic and hybrid search queries.
type SemanticSearcher struct {
	db   *sql.DB
	ai   *ai.Provider
}

// NewSemanticSearcher creates a new semantic searcher.
func NewSemanticSearcher(db *sql.DB, ai *ai.Provider) *SemanticSearcher {
	return &SemanticSearcher{
		db: db,
		ai: ai,
	}
}

// EmbeddingModel returns the model name to use for query embeddings.
func (s *SemanticSearcher) EmbeddingModel() string {
	return "text-embedding-ada-002"
}

// SemanticSearch performs pure semantic similarity search.
func (s *SemanticSearcher) SemanticSearch(ctx context.Context, query string, workspaceID int64, limit int) ([]SearchResult, error) {
	// Generate embedding for the query
	queryEmbedding, err := s.ai.GenerateEmbedding(s.EmbeddingModel(), query)
	if err != nil {
		return nil, fmt.Errorf("generate query embedding: %w", err)
	}

	queryVector := float32SliceToVector(queryEmbedding)

	var results []SearchResult

	// Search messages
	msgResults, err := s.searchMessages(ctx, queryVector, workspaceID, limit/4)
	if err != nil {
		log.Printf("[search] error searching messages: %v", err)
	} else {
		results = append(results, msgResults...)
	}

	// Search posts
	postResults, err := s.searchPosts(ctx, queryVector, workspaceID, limit/4)
	if err != nil {
		log.Printf("[search] error searching posts: %v", err)
	} else {
		results = append(results, postResults...)
	}

	// Search tasks
	taskResults, err := s.searchTasks(ctx, queryVector, workspaceID, limit/4)
	if err != nil {
		log.Printf("[search] error searching tasks: %v", err)
	} else {
		results = append(results, taskResults...)
	}

	// Search files
	fileResults, err := s.searchFiles(ctx, queryVector, workspaceID, limit/4)
	if err != nil {
		log.Printf("[search] error searching files: %v", err)
	} else {
		results = append(results, fileResults...)
	}

	// Sort by score (descending)
	sortByScore(results)

	// Limit results
	if len(results) > limit {
		results = results[:limit]
	}

	return results, nil
}

// HybridSearch performs keyword + semantic combined search.
func (s *SemanticSearcher) HybridSearch(ctx context.Context, query string, workspaceID int64, limit int) ([]SearchResult, error) {
	// For hybrid search, we:
	// 1. Get semantic results
	// 2. Get keyword results (using existing search.go)
	// 3. Combine and re-rank

	semanticResults, err := s.SemanticSearch(ctx, query, workspaceID, limit*2)
	if err != nil {
		return nil, err
	}

	// TODO: Get keyword results from existing search.go
	// For now, return semantic results only

	return semanticResults, nil
}

// searchMessages performs semantic search on messages.
func (s *SemanticSearcher) searchMessages(ctx context.Context, queryVector string, workspaceID int64, limit int) ([]SearchResult, error) {
	queryStr := `
		SELECT
			m.id,
			m.content,
			COALESCE(u.display_name, u.username, 'Unknown') as author_name,
			m.created_at,
			1 - (m.embedding <=> $1::vector) as similarity
		FROM messages m
		LEFT JOIN users u ON m.author_id = u.id
		JOIN channels c ON m.channel_id = c.id
		WHERE c.workspace_id = $2
			AND m.embedding IS NOT NULL
			AND m.is_system = false
		ORDER BY m.embedding <=> $1::vector
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, queryStr, queryVector, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var id int64
		var content, author, date string
		var similarity float64
		if err := rows.Scan(&id, &content, &author, &date, &similarity); err != nil {
			continue
		}

		preview := content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}

		results = append(results, SearchResult{
			Type:        "message",
			ID:          id,
			Title:       content[:50] + "...",
			Preview:     preview,
			Author:      author,
			Date:        date,
			WorkspaceID: workspaceID,
			Score:       similarity,
		})
	}

	return results, nil
}

// searchPosts performs semantic search on posts.
func (s *SemanticSearcher) searchPosts(ctx context.Context, queryVector string, workspaceID int64, limit int) ([]SearchResult, error) {
	queryStr := `
		SELECT
			p.id,
			p.content,
			COALESCE(u.display_name, u.username, 'Unknown') as author_name,
			p.created_at,
			1 - (p.embedding <=> $1::vector) as similarity
		FROM posts p
		LEFT JOIN users u ON p.author_id = u.id
		WHERE p.workspace_id = $2
			AND p.embedding IS NOT NULL
		ORDER BY p.embedding <=> $1::vector
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, queryStr, queryVector, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var id int64
		var content, author, date string
		var similarity float64
		if err := rows.Scan(&id, &content, &author, &date, &similarity); err != nil {
			continue
		}

		preview := content
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}

		results = append(results, SearchResult{
			Type:        "post",
			ID:          id,
			Title:       content[:50] + "...",
			Preview:     preview,
			Author:      author,
			Date:        date,
			WorkspaceID: workspaceID,
			Score:       similarity,
		})
	}

	return results, nil
}

// searchTasks performs semantic search on tasks.
func (s *SemanticSearcher) searchTasks(ctx context.Context, queryVector string, workspaceID int64, limit int) ([]SearchResult, error) {
	queryStr := `
		SELECT
			t.id,
			t.title,
			t.description,
			t.created_at,
			1 - (t.embedding <=> $1::vector) as similarity
		FROM tasks t
		WHERE t.workspace_id = $2
			AND t.embedding IS NOT NULL
		ORDER BY t.embedding <=> $1::vector
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, queryStr, queryVector, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var id int64
		var title, description, date string
		var similarity float64
		if err := rows.Scan(&id, &title, &description, &date, &similarity); err != nil {
			continue
		}

		preview := title
		if description != "" {
			preview = title + ": " + description
		}
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}

		results = append(results, SearchResult{
			Type:        "task",
			ID:          id,
			Title:       title,
			Preview:     preview,
			Date:        date,
			WorkspaceID: workspaceID,
			Score:       similarity,
		})
	}

	return results, nil
}

// searchFiles performs semantic search on files.
func (s *SemanticSearcher) searchFiles(ctx context.Context, queryVector string, workspaceID int64, limit int) ([]SearchResult, error) {
	queryStr := `
		SELECT
			f.id,
			f.filename,
			f.description,
			f.created_at,
			1 - (f.embedding <=> $1::vector) as similarity
		FROM files f
		WHERE f.workspace_id = $2
			AND f.embedding IS NOT NULL
		ORDER BY f.embedding <=> $1::vector
		LIMIT $3
	`

	rows, err := s.db.QueryContext(ctx, queryStr, queryVector, workspaceID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []SearchResult
	for rows.Next() {
		var id int64
		var filename, description, date string
		var similarity float64
		if err := rows.Scan(&id, &filename, &description, &date, &similarity); err != nil {
			continue
		}

		preview := filename
		if description != "" {
			preview = filename + ": " + description
		}
		if len(preview) > 200 {
			preview = preview[:200] + "..."
		}

		results = append(results, SearchResult{
			Type:        "file",
			ID:          id,
			Title:       filename,
			Preview:     preview,
			Date:        date,
			WorkspaceID: workspaceID,
			Score:       similarity,
		})
	}

	return results, nil
}

// sortByScore sorts results by score in descending order.
func sortByScore(results []SearchResult) {
	// Simple bubble sort (fine for small result sets)
	n := len(results)
	for i := 0; i < n-1; i++ {
		for j := 0; j < n-i-1; j++ {
			if results[j].Score < results[j+1].Score {
				results[j], results[j+1] = results[j+1], results[j]
			}
		}
	}
}


// UpdateMessageEmbedding updates the embedding for a single message.
func (s *SemanticSearcher) UpdateMessageEmbedding(ctx context.Context, messageID int64, content string) error {
	embedding, err := s.ai.GenerateEmbedding(s.EmbeddingModel(), content)
	if err != nil {
		return err
	}
	vectorStr := float32SliceToVector(embedding)
	_, err = s.db.ExecContext(ctx, "UPDATE messages SET embedding = $1 WHERE id = $2", vectorStr, messageID)
	return err
}

// UpdatePostEmbedding updates the embedding for a single post.
func (s *SemanticSearcher) UpdatePostEmbedding(ctx context.Context, postID int64, content string) error {
	embedding, err := s.ai.GenerateEmbedding(s.EmbeddingModel(), content)
	if err != nil {
		return err
	}
	vectorStr := float32SliceToVector(embedding)
	_, err = s.db.ExecContext(ctx, "UPDATE posts SET embedding = $1 WHERE id = $2", vectorStr, postID)
	return err
}

// UpdateTaskEmbedding updates the embedding for a single task.
func (s *SemanticSearcher) UpdateTaskEmbedding(ctx context.Context, taskID int64, title, description string) error {
	content := title
	if description != "" {
		content = title + ": " + description
	}
	embedding, err := s.ai.GenerateEmbedding(s.EmbeddingModel(), content)
	if err != nil {
		return err
	}
	vectorStr := float32SliceToVector(embedding)
	_, err = s.db.ExecContext(ctx, "UPDATE tasks SET embedding = $1 WHERE id = $2", vectorStr, taskID)
	return err
}

// UpdateFileEmbedding updates the embedding for a single file.
func (s *SemanticSearcher) UpdateFileEmbedding(ctx context.Context, fileID int64, filename, description string) error {
	content := filename
	if description != "" {
		content = filename + ": " + description
	}
	embedding, err := s.ai.GenerateEmbedding(s.EmbeddingModel(), content)
	if err != nil {
		return err
	}
	vectorStr := float32SliceToVector(embedding)
	_, err = s.db.ExecContext(ctx, "UPDATE files SET embedding = $1 WHERE id = $2", vectorStr, fileID)
	return err
}
