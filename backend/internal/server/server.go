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
	r.Use(corsMiddleware)

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

		r.Get("/images/{imageID}/file", a.GetImageFile)
		r.Post("/images/{imageID}/analysis", a.RunImageAnalysis)
		r.Get("/images/{imageID}/analysis", a.GetImageAnalysis)
	})

	return r
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (a *App) Start(port string) error {
	log.Println("backend started on port", port)

	return http.ListenAndServe(":"+port, a.Routes())
}
