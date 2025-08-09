package api

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
)

func SetupRoutes(app *fiber.App, db *sql.DB, allowUnsafeDuplicateIngest bool) {
	handlers := NewHandlers(db, allowUnsafeDuplicateIngest)

	// Health check endpoint
	app.Get("/healthz", handlers.GetHealthz)

	// Ingest endpoint
	app.Post("/ingest/xlsx", handlers.PostIngestXLSX)

	// Vessel endpoints
	app.Get("/vessels", handlers.GetVessels)
	app.Get("/vessels/:id", handlers.GetVessel)
	app.Get("/vessels/:id/telemetry", handlers.GetVesselTelemetry)
	app.Get("/vessels/:id/latest", handlers.GetVesselLatest)

	// Upload endpoints
	app.Get("/uploads/:id", handlers.GetUpload)

	// OpenAPI endpoint
	app.Get("/.well-known/openapi.json", handlers.GetOpenAPI)
}
