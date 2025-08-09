package main

import (
	"log"
	"os"

	"vessel-telemetry-api/internal/app"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	dbPath := os.Getenv("DB_PATH")
	if dbPath == "" {
		dbPath = "./data/telemetry.db"
	}

	allowUnsafeDuplicateIngest := os.Getenv("ALLOW_UNSAFE_DUPLICATE_INGEST") == "true"

	app, err := app.New(dbPath, allowUnsafeDuplicateIngest)
	if err != nil {
		log.Fatal("Failed to initialize app:", err)
	}
	defer app.Close()

	log.Printf("Starting server on port %s", port)
	log.Fatal(app.Listen(":" + port))
}
