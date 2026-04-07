package app

import (
	"fmt"
	"html/template"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

var funcMap = template.FuncMap{
	"timeago":        timeAgo,
	"datetime":       func(t time.Time) string { return t.Format("Jan 2, 2006 3:04 PM") },
	"indent":         func(depth int) int { return depth * 24 },
	"raw":            func(s string) template.HTML { return template.HTML(s) },
	"add":            func(a, b int) int { return a + b },
	"shouldtruncate": shouldTruncatePostContent,
	"asset":          assetURL,
}

func timeAgo(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	case d < 30*24*time.Hour:
		days := int(math.Floor(d.Hours() / 24))
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	default:
		return t.Format("Jan 2, 2006")
	}
}

func shouldTruncatePostContent(content string) bool {
	if len(content) > 900 {
		return true
	}
	if strings.Count(content, "\n") >= 14 {
		return true
	}
	if strings.Contains(content, "```") || strings.Contains(content, "\n#") {
		return true
	}
	if strings.Contains(content, "![](") || strings.Contains(content, "![") {
		return true
	}
	return false
}

func assetURL(path string) string {
	localPath := strings.TrimPrefix(path, "/")
	info, err := os.Stat(projectPath(localPath))
	if err != nil {
		return path
	}
	return fmt.Sprintf("%s?v=%d", path, info.ModTime().Unix())
}

func LoadTemplates() map[string]*template.Template {
	templates := make(map[string]*template.Template)

	layout := projectPath("templates", "layout.html")
	fragment := projectPath("templates", "_postfragment.html")
	pages, _ := filepath.Glob(projectPath("templates", "*.html"))

	for _, page := range pages {
		name := filepath.Base(page)
		if name == "layout.html" || strings.HasPrefix(name, "_") {
			continue
		}
		t, err := template.New("").Funcs(funcMap).ParseFiles(layout, fragment, page)
		if err != nil {
			panic(fmt.Sprintf("parse template %s: %v", name, err))
		}
		templates[name] = t
	}

	// Parse fragment template standalone for htmx partial rendering
	t, err := template.New("").Funcs(funcMap).ParseFiles(fragment)
	if err != nil {
		panic(fmt.Sprintf("parse fragment template: %v", err))
	}
	templates["_postfragment.html"] = t

	return templates
}

func (app *App) render(w http.ResponseWriter, name string, data *PageData) {
	t, ok := app.Templates[name]
	if !ok {
		http.Error(w, fmt.Sprintf("template %q not found", name), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

// renderFragment renders just the post list fragment for htmx pagination requests.
func (app *App) renderFragment(w http.ResponseWriter, data *PageData) {
	t, ok := app.Templates["_postfragment.html"]
	if !ok {
		http.Error(w, "fragment template not found", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := t.ExecuteTemplate(w, "postfragment", data); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (app *App) renderStatus(w http.ResponseWriter, r *http.Request, statusCode int, title, message string) {
	if r != nil && r.Header.Get("HX-Request") == "true" {
		http.Error(w, message, statusCode)
		return
	}

	data := &PageData{
		CurrentUser:  GetCurrentUser(r),
		Title:        title,
		StatusCode:   statusCode,
		ErrorTitle:   title,
		ErrorMessage: message,
	}

	t, ok := app.Templates["error.html"]
	if !ok {
		http.Error(w, message, statusCode)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(statusCode)
	if err := t.ExecuteTemplate(w, "layout", data); err != nil {
		http.Error(w, message, statusCode)
	}
}

func (app *App) renderNotFound(w http.ResponseWriter, r *http.Request) {
	app.renderStatus(w, r, http.StatusNotFound, "Page Not Found", "That page does not exist.")
}
