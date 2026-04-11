package app

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// --- Settings ---

func (app *App) handleGetSettings(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if !app.isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}
	rows, err := app.DB.Query(`SELECT key, value FROM settings ORDER BY key`)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load settings")
		return
	}
	defer rows.Close()

	settings := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		settings[k] = v
	}
	writeJSON(w, http.StatusOK, settings)
}

func (app *App) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	user := UserFromContext(r)
	if !app.isAdmin(user) {
		writeError(w, http.StatusForbidden, "admin access required")
		return
	}

	var input map[string]string
	if err := readJSON(r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	for key, value := range input {
		if !isValidSettingKey(key) {
			continue
		}
		value = validateSettingValue(key, value)
		app.DB.Exec(`
			INSERT INTO settings (key, value, updated_at) VALUES ($1, $2, $3)
			ON CONFLICT (key) DO UPDATE SET value = $2, updated_at = $3
		`, key, value, time.Now())
	}

	// Update AI provider model if changed
	if model, ok := input["ai_model"]; ok {
		app.AI.SetModel(model)
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
}

func (app *App) handleGetModels(w http.ResponseWriter, r *http.Request) {
	models, err := app.AI.GetModels()
	if err != nil {
		log.Printf("[admin] failed to fetch models: %v", err)
		writeError(w, http.StatusInternalServerError, "failed to fetch models")
		return
	}
	if models == nil {
		models = []string{}
	}
	writeJSON(w, http.StatusOK, models)
}

func (app *App) handleHealthCheck(w http.ResponseWriter, r *http.Request) {
	services := map[string]string{}

	// Check LiteLLM
	if err := app.AI.Health(); err != nil {
		log.Printf("[health] litellm: %v", err)
		services["litellm"] = "down"
	} else {
		services["litellm"] = "ok"
	}

	// Check MinIO
	if app.Storage != nil {
		if err := app.Storage.Health(r.Context()); err != nil {
			log.Printf("[health] minio: %v", err)
			services["minio"] = "down"
		} else {
			services["minio"] = "ok"
		}
	} else {
		services["minio"] = "not configured"
	}

	// Check PostgreSQL
	if err := app.DB.Ping(); err != nil {
		services["postgres"] = "down"
	} else {
		services["postgres"] = "ok"
	}

	// Check ChromaDB
	if app.RAG != nil {
		if err := app.RAG.Health(); err != nil {
			log.Printf("[health] chromadb: %v", err)
			services["chromadb"] = "down"
		} else {
			services["chromadb"] = "ok"
		}
	} else {
		services["chromadb"] = "not configured"
	}

	status := http.StatusOK
	for _, v := range services {
		if strings.HasPrefix(v, "down") {
			status = http.StatusServiceUnavailable
			break
		}
	}
	writeJSON(w, status, services)
}

func isValidSettingKey(key string) bool {
	validKeys := map[string]bool{
		"ai_model":                true,
		"ai_fallback_model":       true,
		"ai_system_prompt":        true,
		"ai_max_context_messages": true,
		"ai_temperature":          true,
	}
	return validKeys[key]
}

func validateSettingValue(key, value string) string {
	switch key {
	case "ai_system_prompt":
		if len(value) > 5000 {
			return value[:5000]
		}
	case "ai_temperature":
		// Clamp to valid range
		t := 0.7
		for _, c := range value {
			if c == '.' || (c >= '0' && c <= '9') {
				continue
			}
			break
		}
		if _, err := strconv.ParseFloat(value, 64); err != nil {
			return "0.7"
		}
	case "ai_max_context_messages":
		if _, err := strconv.Atoi(value); err != nil {
			return "50"
		}
	}
	return value
}

func (app *App) isAdmin(user *User) bool {
	if user == nil {
		return false
	}
	var role string
	err := app.DB.QueryRow(`SELECT role FROM workspace_members WHERE user_id = $1 LIMIT 1`, user.ID).Scan(&role)
	if err != nil {
		return false
	}
	return role == "owner" || role == "admin"
}

func (app *App) getAISetting(key string, fallback string) string {
	var value string
	err := app.DB.QueryRow(`SELECT value FROM settings WHERE key = $1`, key).Scan(&value)
	if err != nil || value == "" {
		return fallback
	}
	return value
}
