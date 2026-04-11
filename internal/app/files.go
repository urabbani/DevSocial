package app

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"devsocial/internal/rag"
	"devsocial/internal/storage"
)

// --- File Models ---

type File struct {
	ID          int64  `json:"id"`
	WorkspaceID int64  `json:"workspace_id"`
	UploaderID  int64  `json:"uploader_id"`
	Filename    string `json:"filename"`
	S3Key       string `json:"s3_key"`
	ContentType string `json:"content_type"`
	SizeBytes   int64  `json:"size_bytes"`
	CreatedAt   string `json:"created_at"`
	// Joined
	UploaderName string `json:"uploader_name,omitempty"`
}

// --- File Handlers ---

func (app *App) handleUpload(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if user == nil {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	// Parse multipart form (max 50MB)
	if err := r.ParseMultipartForm(50 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "failed to parse form: "+err.Error())
		return
	}

	workspaceIDStr := r.FormValue("workspace_id")
	if workspaceIDStr == "" {
		writeError(w, http.StatusBadRequest, "workspace_id is required")
		return
	}
	workspaceID, err := strconv.ParseInt(workspaceIDStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace_id")
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "file is required")
		return
	}
	defer file.Close()

	contentType := header.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}

	// Validate file type
	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !isAllowedFileType(ext) {
		writeError(w, http.StatusBadRequest, "file type not allowed: "+ext)
		return
	}

	// Upload to MinIO
	s3Key := storage.MakeS3Key(workspaceID, header.Filename)
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	if app.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	if ok, _ := app.IsWorkspaceMember(workspaceID, user.ID); !ok {
		writeError(w, http.StatusForbidden, "not a member of this workspace")
		return
	}

	if err := app.Storage.Upload(ctx, s3Key, file, header.Size, contentType); err != nil {
		writeError(w, http.StatusInternalServerError, "upload failed: "+err.Error())
		return
	}

	// Save metadata
	var f File
	err = app.DB.QueryRow(`
		INSERT INTO files (workspace_id, uploader_id, filename, s3_key, content_type, size_bytes)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, workspace_id, uploader_id, filename, s3_key, content_type, size_bytes, created_at
	`, workspaceID, user.ID, header.Filename, s3Key, contentType, header.Size).Scan(
		&f.ID, &f.WorkspaceID, &f.UploaderID, &f.Filename, &f.S3Key,
		&f.ContentType, &f.SizeBytes, &f.CreatedAt,
	)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to save file metadata")
		return
	}
	f.UploaderName = user.Username

	// Trigger RAG indexing in background if applicable
	if app.RAG != nil && isDocumentType(ext) {
		go app.indexFile(f.ID, workspaceID, s3Key, header.Filename)
	}

	writeJSON(w, http.StatusCreated, f)
}

func (app *App) handleListFiles(w http.ResponseWriter, r *http.Request) {
	workspaceID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid workspace id")
		return
	}

	rows, err := app.DB.Query(`
		SELECT f.id, f.workspace_id, f.uploader_id, f.filename, f.s3_key, f.content_type, f.size_bytes, f.created_at,
		       u.username
		FROM files f
		JOIN users u ON u.id = f.uploader_id
		WHERE f.workspace_id = $1
		ORDER BY f.created_at DESC
	`, workspaceID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to list files")
		return
	}
	defer rows.Close()

	var files []File
	for rows.Next() {
		var f File
		if err := rows.Scan(&f.ID, &f.WorkspaceID, &f.UploaderID, &f.Filename, &f.S3Key,
			&f.ContentType, &f.SizeBytes, &f.CreatedAt, &f.UploaderName); err != nil {
			continue
		}
		files = append(files, f)
	}
	if files == nil {
		files = []File{}
	}
	writeJSON(w, http.StatusOK, files)
}

func (app *App) handleGetFile(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var f File
	err = app.DB.QueryRow(`
		SELECT id, workspace_id, uploader_id, filename, s3_key, content_type, size_bytes, created_at
		FROM files WHERE id = $1
	`, id).Scan(&f.ID, &f.WorkspaceID, &f.UploaderID, &f.Filename, &f.S3Key,
		&f.ContentType, &f.SizeBytes, &f.CreatedAt)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	if app.Storage == nil {
		writeError(w, http.StatusServiceUnavailable, "file storage not configured")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()

	obj, err := app.Storage.Download(ctx, f.S3Key)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to download file")
		return
	}
	defer obj.Close()

	w.Header().Set("Content-Type", f.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename*=UTF-8''%s`, url.PathEscape(f.Filename)))
	w.Header().Set("Cache-Control", "private, max-age=3600")

	stat, err := obj.Stat()
	if err == nil {
		w.Header().Set("Content-Length", strconv.FormatInt(stat.Size, 10))
	}
	io.Copy(w, obj)
}

func (app *App) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid file id")
		return
	}

	var s3Key string
	var uploaderID int64
	err = app.DB.QueryRow(`SELECT s3_key, uploader_id FROM files WHERE id = $1`, id).Scan(&s3Key, &uploaderID)
	if err != nil {
		writeError(w, http.StatusNotFound, "file not found")
		return
	}

	// Only uploader or admin can delete
	if uploaderID != user.ID {
		writeError(w, http.StatusForbidden, "not authorized to delete this file")
		return
	}

	if app.Storage != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		app.Storage.Delete(ctx, s3Key)
	}

	app.DB.Exec(`DELETE FROM files WHERE id = $1`, id)
	writeJSON(w, http.StatusOK, map[string]string{"status": "deleted"})
}

// --- Helpers ---

func isAllowedFileType(ext string) bool {
	allowed := map[string]bool{
		".pdf": true, ".txt": true, ".md": true,
		".csv": true, ".tsv": true, ".xlsx": true, ".xls": true,
		".json": true, ".jsonl": true, ".xml": true, ".yaml": true, ".yml": true,
		".py": true, ".js": true, ".ts": true, ".go": true, ".rs": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".r": true, ".R": true, ".sql": true, ".sh": true, ".bash": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".webp": true, ".svg": true,
		".docx": true, ".doc": true, ".pptx": true, ".ppt": true,
		".zip": true, ".tar": true, ".gz": true,
		".ipynb": true, ".html": true, ".css": true,
	}
	return allowed[ext]
}

func isDocumentType(ext string) bool {
	docs := map[string]bool{
		".pdf": true, ".txt": true, ".md": true,
		".csv": true, ".json": true, ".jsonl": true, ".xml": true, ".yaml": true, ".yml": true,
		".py": true, ".js": true, ".ts": true, ".go": true, ".rs": true, ".java": true,
		".c": true, ".cpp": true, ".h": true, ".hpp": true,
		".r": true, ".R": true, ".sql": true, ".sh": true,
		".docx": true, ".ipynb": true, ".html": true,
	}
	return docs[ext]
}

func (app *App) indexFile(fileID, workspaceID int64, s3Key, filename string) {
	log.Printf("[rag] indexing file %d: %s (workspace %d)", fileID, filename, workspaceID)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// Download from MinIO
	if app.Storage == nil {
		log.Printf("[rag] storage not available, skipping")
		return
	}
	obj, err := app.Storage.Download(ctx, s3Key)
	if err != nil {
		log.Printf("[rag] failed to download file %d: %v", fileID, err)
		return
	}
	defer obj.Close()

	data, err := io.ReadAll(io.LimitReader(obj, 10<<20)) // 10MB max for indexing
	if err != nil {
		log.Printf("[rag] failed to read file %d: %v", fileID, err)
		return
	}

	// Extract text based on file type
	ext := strings.ToLower(filepath.Ext(filename))
	text := app.extractText(ext, data)
	if text == "" {
		log.Printf("[rag] no text extracted from %s, skipping", filename)
		return
	}

	// Truncate very large files
	if len(text) > 50000 {
		text = text[:50000]
	}

	// Split into chunks (~1000 chars with overlap)
	chunks := chunkText(text, 1000, 200)
	if len(chunks) == 0 {
		return
	}

	// Generate embeddings for each chunk
	embeddingModel := "text-embedding-3-small"
	var docs []rag.Document
	for i, chunk := range chunks {
		emb, err := app.AI.GenerateEmbedding(embeddingModel, chunk)
		if err != nil {
			log.Printf("[rag] embedding failed for chunk %d: %v", i, err)
			continue
		}
		docs = append(docs, rag.Document{
			ID:        fmt.Sprintf("file-%d-chunk-%d", fileID, i),
			Content:   chunk,
			Embedding: emb,
			Metadata: map[string]string{
				"file_id":     fmt.Sprintf("%d", fileID),
				"workspace_id": fmt.Sprintf("%d", workspaceID),
				"filename":    filename,
				"chunk_index": fmt.Sprintf("%d", i),
			},
		})
	}

	if err := app.RAG.AddDocuments("devsocial_docs", docs); err != nil {
		log.Printf("[rag] failed to index file %d: %v", fileID, err)
		return
	}
	log.Printf("[rag] indexed file %d: %d chunks", fileID, len(chunks))
}

// extractText pulls text content from various file types.
func (app *App) extractText(ext string, data []byte) string {
	switch ext {
	case ".txt", ".md", ".csv", ".tsv", ".json", ".jsonl", ".xml", ".yaml", ".yml", ".sql", ".sh", ".bash":
		return string(data)
	case ".py", ".js", ".ts", ".go", ".rs", ".java", ".c", ".cpp", ".h", ".hpp", ".r", ".html", ".css":
		return string(data)
	case ".ipynb":
		// Extract code cells from Jupyter notebooks
		return extractNotebookText(data)
	default:
		return ""
	}
}

// extractNotebookText extracts code and text from Jupyter notebook JSON.
func extractNotebookText(data []byte) string {
	var nb struct {
		Cells []struct {
			CellType string `json:"cell_type"`
			Source   any    `json:"source"`
		} `json:"cells"`
	}
	if err := json.Unmarshal(data, &nb); err != nil {
		return ""
	}
	var buf strings.Builder
	for _, cell := range nb.Cells {
		switch src := cell.Source.(type) {
		case string:
			buf.WriteString(src)
		case []any:
			for _, s := range src {
				buf.WriteString(fmt.Sprintf("%v", s))
			}
		}
		buf.WriteString("\n\n")
	}
	return strings.TrimSpace(buf.String())
}

// chunkText splits text into overlapping chunks.
func chunkText(text string, chunkSize, overlap int) []string {
	if len(text) <= chunkSize {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}
	var chunks []string
	runes := []rune(text)
	for i := 0; i < len(runes); i += chunkSize - overlap {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		chunk := string(runes[i:end])
		if strings.TrimSpace(chunk) != "" {
			chunks = append(chunks, chunk)
		}
		if end >= len(runes) {
			break
		}
	}
	return chunks
}
