package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"devsocial/internal/app"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbPath := flag.String("db", "devsocial.db", "SQLite database path")
	flag.Parse()

	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	baseURL := os.Getenv("BASE_URL")

	if githubClientID == "" || githubClientSecret == "" {
		log.Fatal("GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET environment variables are required")
	}
	if baseURL == "" {
		baseURL = "http://localhost" + *addr
	}

	db, err := app.InitDB(*dbPath)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	application := app.New(db, githubClientID, githubClientSecret, baseURL)

	log.Printf("DevSocial starting on %s", *addr)
	if err := http.ListenAndServe(*addr, application.Handler()); err != nil {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("DevSocial stopped")
}
