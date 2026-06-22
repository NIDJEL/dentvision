package server

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type App struct {
	db           *pgxpool.Pool
	jwtSecret    string
	uploadsDir   string
	mlServiceURL string
}

func New(db *pgxpool.Pool, jwtSecret string, uploadsDir string, mlServiceURL string) *App {
	if mlServiceURL == "" {
		mlServiceURL = "http://ml-service:8000"
	}

	return &App{
		db:           db,
		jwtSecret:    jwtSecret,
		uploadsDir:   uploadsDir,
		mlServiceURL: mlServiceURL,
	}
}

func (a *App) Routes() http.Handler {
	r := chi.NewRouter()

	r.Get("/health", a.Health)

	r.Route("/auth", func(r chi.Router) {
		r.Post("/login", a.Login)
	})

	r.Group(func(r chi.Router) {
		r.Use(a.AuthMiddleware)

		r.Get("/me", a.Me)

		r.Post("/patients", a.CreatePatient)
		r.Get("/patients", a.ListPatients)
		r.Get("/patients/{patientID}", a.GetPatient)

		r.Post("/patients/{patientID}/images", a.UploadPatientImage)
		r.Get("/patients/{patientID}/images", a.ListPatientImages)

		r.Post("/images/{imageID}/analysis", a.RunImageAnalysis)
		r.Get("/images/{imageID}/analysis", a.GetImageAnalysis)
	})

	return r
}

func (a *App) Start(port string) error {
	log.Println("backend started on port", port)

	return http.ListenAndServe(":"+port, a.Routes())
}
