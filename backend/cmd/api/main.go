package main

import (
	"log"

	"github.com/NIDJEL/dentvision/backend/internal/config"
	"github.com/NIDJEL/dentvision/backend/internal/database"
	"github.com/NIDJEL/dentvision/backend/internal/server"
)

func main() {
	cfg := config.Load()

	db, err := database.NewPostgresPool(cfg.DatabaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	app := server.New(db, cfg.JWTSecret, cfg.UploadsDir)

	if err := app.Start(cfg.Port); err != nil {
		log.Fatal(err)
	}
}
