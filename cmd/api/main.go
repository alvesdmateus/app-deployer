package main

import (
	"log"
	"os"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/rs/zerolog"
)

func main() {
	// Initialize logger
	zlog := zerolog.New(os.Stdout).With().Timestamp().Logger()
	zlog.Info().Msg("Starting app-deployer API server")

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		AppName: "app-deployer API v0.1.0",
	})

	// Middleware
	app.Use(recover.New())
	app.Use(logger.New())

	// Health check endpoints
	app.Get("/health/live", func(c *fiber.Ctx) error {
		return c.JSON(fiber.Map{
			"status": "alive",
		})
	})

	app.Get("/health/ready", func(c *fiber.Ctx) error {
		// TODO: Check database connection, redis connection
		return c.JSON(fiber.Map{
			"status": "ready",
		})
	})

	// API routes
	api := app.Group("/api/v1")

	// Deployments endpoints (placeholder)
	api.Post("/deployments", func(c *fiber.Ctx) error {
		return c.Status(501).JSON(fiber.Map{
			"error": "Not implemented yet",
		})
	})

	api.Get("/deployments/:id", func(c *fiber.Ctx) error {
		return c.Status(501).JSON(fiber.Map{
			"error": "Not implemented yet",
		})
	})

	api.Delete("/deployments/:id", func(c *fiber.Ctx) error {
		return c.Status(501).JSON(fiber.Map{
			"error": "Not implemented yet",
		})
	})

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	zlog.Info().Str("port", port).Msg("API server listening")
	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}
}
