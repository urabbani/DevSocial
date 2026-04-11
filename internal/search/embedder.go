package search

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"strings"

	"devsocial/internal/ai"
)

// Embedder handles batch embedding of existing content for semantic search.
type Embedder struct {
	db     *sql.DB
	ai     *ai.Provider
	batchSize int
}

// NewEmbedder creates a new embedder for migrating existing content.
func NewEmbedder(db *sql.DB, ai *ai.Provider) *Embedder {
	return &Embedder{
		db:     db,
		ai:     ai,
		batchSize: 50, // Process 50 records at a time
	}
}

// EmbeddingModel returns the model name to use for embeddings.
func (e *Embedder) EmbeddingModel() string {
	return "text-embedding-ada-002" // Default LiteLLM embedding model
}

// MigrateMessages generates embeddings for all messages without embeddings.
func (e *Embedder) MigrateMessages(ctx context.Context) error {
	log.Println("[embedder] starting message embedding migration")

	// Find messages without embeddings
	rows, err := e.db.QueryContext(ctx, `
		SELECT id, content FROM messages
		WHERE embedding IS NULL AND content IS NOT NULL AND content != ''
		ORDER BY created_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query messages: %w", err)
	}
	defer rows.Close()

	var toEmbed []struct {
		ID      int64
		Content string
	}

	for rows.Next() {
		var id int64
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Printf("[embedder] error scanning message: %v", err)
			continue
		}
		toEmbed = append(toEmbed, struct {
			ID      int64
			Content string
		}{id, content})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate messages: %w", err)
	}

	log.Printf("[embedder] found %d messages to embed", len(toEmbed))

	// Process in batches
	for i := 0; i < len(toEmbed); i += e.batchSize {
		end := i + e.batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		if err := e.embedMessageBatch(ctx, batch); err != nil {
			log.Printf("[embedder] error embedding batch %d: %v", i/e.batchSize, err)
		}
	}

	log.Println("[embedder] message embedding migration complete")
	return nil
}

// embedMessageBatch embeds a batch of messages.
func (e *Embedder) embedMessageBatch(ctx context.Context, batch []struct {
	ID      int64
	Content string
}) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE messages SET embedding = $2 WHERE id = $1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range batch {
		embedding, err := e.ai.GenerateEmbedding(e.EmbeddingModel(), item.Content)
		if err != nil {
			log.Printf("[embedder] error embedding message %d: %v", item.ID, err)
			continue
		}

		// Convert []float32 to pgvector format string
		vectorStr := float32SliceToVector(embedding)
		if _, err := stmt.Exec(item.ID, vectorStr); err != nil {
			log.Printf("[embedder] error updating message %d: %v", item.ID, err)
		}
	}

	return tx.Commit()
}

// MigratePosts generates embeddings for all posts without embeddings.
func (e *Embedder) MigratePosts(ctx context.Context) error {
	log.Println("[embedder] starting post embedding migration")

	rows, err := e.db.QueryContext(ctx, `
		SELECT id, content FROM posts
		WHERE embedding IS NULL AND content IS NOT NULL AND content != ''
		ORDER BY created_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query posts: %w", err)
	}
	defer rows.Close()

	var toEmbed []struct {
		ID      int64
		Content string
	}

	for rows.Next() {
		var id int64
		var content string
		if err := rows.Scan(&id, &content); err != nil {
			log.Printf("[embedder] error scanning post: %v", err)
			continue
		}
		toEmbed = append(toEmbed, struct {
			ID      int64
			Content string
		}{id, content})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate posts: %w", err)
	}

	log.Printf("[embedder] found %d posts to embed", len(toEmbed))

	for i := 0; i < len(toEmbed); i += e.batchSize {
		end := i + e.batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		if err := e.embedPostBatch(ctx, batch); err != nil {
			log.Printf("[embedder] error embedding batch %d: %v", i/e.batchSize, err)
		}
	}

	log.Println("[embedder] post embedding migration complete")
	return nil
}

// embedPostBatch embeds a batch of posts.
func (e *Embedder) embedPostBatch(ctx context.Context, batch []struct {
	ID      int64
	Content string
}) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE posts SET embedding = $2 WHERE id = $1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range batch {
		embedding, err := e.ai.GenerateEmbedding(e.EmbeddingModel(), item.Content)
		if err != nil {
			log.Printf("[embedder] error embedding post %d: %v", item.ID, err)
			continue
		}

		vectorStr := float32SliceToVector(embedding)
		if _, err := stmt.Exec(item.ID, vectorStr); err != nil {
			log.Printf("[embedder] error updating post %d: %v", item.ID, err)
		}
	}

	return tx.Commit()
}

// MigrateTasks generates embeddings for all tasks without embeddings.
func (e *Embedder) MigrateTasks(ctx context.Context) error {
	log.Println("[embedder] starting task embedding migration")

	rows, err := e.db.QueryContext(ctx, `
		SELECT id, title, description FROM tasks
		WHERE embedding IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query tasks: %w", err)
	}
	defer rows.Close()

	var toEmbed []struct {
		ID          int64
		Title       string
		Description string
	}

	for rows.Next() {
		var id int64
		var title, description string
		if err := rows.Scan(&id, &title, &description); err != nil {
			log.Printf("[embedder] error scanning task: %v", err)
			continue
		}
		toEmbed = append(toEmbed, struct {
			ID          int64
			Title       string
			Description string
		}{id, title, description})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate tasks: %w", err)
	}

	log.Printf("[embedder] found %d tasks to embed", len(toEmbed))

	for i := 0; i < len(toEmbed); i += e.batchSize {
		end := i + e.batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		if err := e.embedTaskBatch(ctx, batch); err != nil {
			log.Printf("[embedder] error embedding batch %d: %v", i/e.batchSize, err)
		}
	}

	log.Println("[embedder] task embedding migration complete")
	return nil
}

// embedTaskBatch embeds a batch of tasks.
func (e *Embedder) embedTaskBatch(ctx context.Context, batch []struct {
	ID          int64
	Title       string
	Description string
}) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE tasks SET embedding = $2 WHERE id = $1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range batch {
		// Combine title and description for embedding
		content := item.Title
		if item.Description != "" {
			content = item.Title + ": " + item.Description
		}

		embedding, err := e.ai.GenerateEmbedding(e.EmbeddingModel(), content)
		if err != nil {
			log.Printf("[embedder] error embedding task %d: %v", item.ID, err)
			continue
		}

		vectorStr := float32SliceToVector(embedding)
		if _, err := stmt.Exec(item.ID, vectorStr); err != nil {
			log.Printf("[embedder] error updating task %d: %v", item.ID, err)
		}
	}

	return tx.Commit()
}

// MigrateFiles generates embeddings for all files without embeddings.
func (e *Embedder) MigrateFiles(ctx context.Context) error {
	log.Println("[embedder] starting file embedding migration")

	rows, err := e.db.QueryContext(ctx, `
		SELECT id, filename, description FROM files
		WHERE embedding IS NULL
		ORDER BY created_at DESC
	`)
	if err != nil {
		return fmt.Errorf("query files: %w", err)
	}
	defer rows.Close()

	var toEmbed []struct {
		ID          int64
		Filename    string
		Description string
	}

	for rows.Next() {
		var id int64
		var filename, description string
		if err := rows.Scan(&id, &filename, &description); err != nil {
			log.Printf("[embedder] error scanning file: %v", err)
			continue
		}
		toEmbed = append(toEmbed, struct {
			ID          int64
			Filename    string
			Description string
		}{id, filename, description})
	}

	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate files: %w", err)
	}

	log.Printf("[embedder] found %d files to embed", len(toEmbed))

	for i := 0; i < len(toEmbed); i += e.batchSize {
		end := i + e.batchSize
		if end > len(toEmbed) {
			end = len(toEmbed)
		}
		batch := toEmbed[i:end]

		if err := e.embedFileBatch(ctx, batch); err != nil {
			log.Printf("[embedder] error embedding batch %d: %v", i/e.batchSize, err)
		}
	}

	log.Println("[embedder] file embedding migration complete")
	return nil
}

// embedFileBatch embeds a batch of files.
func (e *Embedder) embedFileBatch(ctx context.Context, batch []struct {
	ID          int64
	Filename    string
	Description string
}) error {
	tx, err := e.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		UPDATE files SET embedding = $2 WHERE id = $1
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range batch {
		// Use filename and description for embedding
		content := item.Filename
		if item.Description != "" {
			content = item.Filename + ": " + item.Description
		}

		embedding, err := e.ai.GenerateEmbedding(e.EmbeddingModel(), content)
		if err != nil {
			log.Printf("[embedder] error embedding file %d: %v", item.ID, err)
			continue
		}

		vectorStr := float32SliceToVector(embedding)
		if _, err := stmt.Exec(item.ID, vectorStr); err != nil {
			log.Printf("[embedder] error updating file %d: %v", item.ID, err)
		}
	}

	return tx.Commit()
}

// MigrateAll runs all embedding migrations.
func (e *Embedder) MigrateAll(ctx context.Context) error {
	log.Println("[embedder] starting full embedding migration")

	if err := e.MigrateMessages(ctx); err != nil {
		return fmt.Errorf("migrate messages: %w", err)
	}
	if err := e.MigratePosts(ctx); err != nil {
		return fmt.Errorf("migrate posts: %w", err)
	}
	if err := e.MigrateTasks(ctx); err != nil {
		return fmt.Errorf("migrate tasks: %w", err)
	}
	if err := e.MigrateFiles(ctx); err != nil {
		return fmt.Errorf("migrate files: %w", err)
	}

	log.Println("[embedder] full embedding migration complete")
	return nil
}

// float32SliceToVector converts a []float32 to pgvector string format.
func float32SliceToVector(v []float32) string {
	if len(v) == 0 {
		return "[]"
	}
	parts := make([]string, len(v))
	for i, f := range v {
		parts[i] = fmt.Sprintf("%f", f)
	}
	return "[" + strings.Join(parts, ",") + "]"
}
