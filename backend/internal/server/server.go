package server

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db        *pgxpool.Pool
	jwtSecret string
}

func New(db *pgxpool.Pool, jwtSecret string) *App {
	return &App{
		db:        db,
		jwtSecret: jwtSecret,
	}
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", a.Health)

	return r
}

func (a *App) Start(port string) error {
	log.Println("backend started on port", port)

	return http.ListenAndServe(":"+port, a.Routes())
}
