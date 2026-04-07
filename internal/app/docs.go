package app

import (
	"fmt"
	"net/http"
	"os"
)

var docsPath = projectPath("docs.md")

func loadDocs() (string, string, error) {
	source, err := os.ReadFile(docsPath)
	if err != nil {
		return "", "", fmt.Errorf("read docs: %w", err)
	}
	markdown := string(source)
	html, err := RenderMarkdown(markdown)
	if err != nil {
		return "", "", fmt.Errorf("render docs: %w", err)
	}
	return markdown, html, nil
}

func (app *App) handleDocs(w http.ResponseWriter, r *http.Request) {
	markdown, html, err := loadDocs()
	if err != nil {
		app.renderStatus(w, r, http.StatusInternalServerError, "Docs Error", "Failed to load docs.")
		return
	}

	app.render(w, "docs.html", &PageData{
		CurrentUser:  GetCurrentUser(r),
		Title:        "Docs",
		BodyClass:    "docs-view",
		DocsHTML:     html,
		DocsMarkdown: markdown,
	})
}

func (app *App) handleDocsRaw(w http.ResponseWriter, r *http.Request) {
	markdown, _, err := loadDocs()
	if err != nil {
		http.Error(w, "Failed to load docs.", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/markdown; charset=utf-8")
	w.Write([]byte(markdown))
}
