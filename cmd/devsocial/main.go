package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"devsocial/internal/app"
)

func main() {
	addr := flag.String("addr", ":8080", "HTTP listen address")
	dbURL := flag.String("db-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection URL")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://devsocial:devsocial@localhost:5432/devsocial?sslmode=disable"
	}

	githubClientID := os.Getenv("GITHUB_CLIENT_ID")
	githubClientSecret := os.Getenv("GITHUB_CLIENT_SECRET")
	baseURL := os.Getenv("BASE_URL")

	if githubClientID == "" || githubClientSecret == "" {
		log.Fatal("GITHUB_CLIENT_ID and GITHUB_CLIENT_SECRET environment variables are required")
	}
	if baseURL == "" {
		baseURL = "http://localhost" + *addr
	}

	db, err := app.InitDB(*dbURL)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	application := app.New(db, githubClientID, githubClientSecret, baseURL)

	srv := &http.Server{
		Addr:    *addr,
		Handler: application.Handler(),
	}

	// Graceful shutdown
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh
		log.Println("Shutting down...")
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		srv.Shutdown(ctx)
	}()

	log.Printf("DevSocial starting on %s", *addr)
	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}

	fmt.Println("DevSocial stopped")
}
