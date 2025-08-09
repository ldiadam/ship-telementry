package app

import (
	"database/sql"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"

	"vessel-telemetry-api/internal/api"
	"vessel-telemetry-api/internal/db"
)

type App struct {
	*fiber.App
	db *sql.DB
}

func New(dbPath string, allowUnsafeDuplicateIngest bool) (*App, error) {
	database, err := db.Connect(dbPath)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(database); err != nil {
		return nil, err
	}

	app := fiber.New(fiber.Config{
		ErrorHandler: func(c *fiber.Ctx, err error) error {
			code := fiber.StatusInternalServerError
			if e, ok := err.(*fiber.Error); ok {
				code = e.Code
			}
			return c.Status(code).JSON(fiber.Map{
				"error": err.Error(),
			})
		},
	})

	app.Use(logger.New())
	app.Use(cors.New())

	// Serve static files
	app.Static("/", "./web")

	api.SetupRoutes(app, database, allowUnsafeDuplicateIngest)

	return &App{
		App: app,
		db:  database,
	}, nil
}

func (a *App) Close() error {
	return a.db.Close()
}
