package server

import (
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type analysisResultDTO struct {
	ID         int64     `json:"id"`
	JobID      int64     `json:"job_id"`
	ImageID    int64     `json:"image_id"`
	Label      string    `json:"label"`
	Confidence float64   `json:"confidence"`
	X          int       `json:"x"`
	Y          int       `json:"y"`
	Width      int       `json:"width"`
	Height     int       `json:"height"`
	CreatedAt  time.Time `json:"created_at"`
}

type analysisJobDTO struct {
	ID         int64               `json:"id"`
	ImageID    int64               `json:"image_id"`
	ModelID    int64               `json:"model_id"`
	Status     string              `json:"status"`
	CreatedAt  time.Time           `json:"created_at"`
	FinishedAt string              `json:"finished_at"`
	Results    []analysisResultDTO `json:"results"`
}

func (a *App) RunImageAnalysis(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	imageID, err := strconv.ParseInt(chi.URLParam(r, "imageID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	if !a.imageBelongsToDoctor(r, imageID, claims.UserID) {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		log.Println("begin analysis tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(r.Context())

	var modelID int64

	err = tx.QueryRow(
		r.Context(),
		`
		SELECT id
		FROM analysis_models
		WHERE is_active = TRUE
		ORDER BY created_at DESC
		LIMIT 1
		`,
	).Scan(&modelID)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusBadRequest, "active analysis model not found")
			return
		}

		log.Println("select active model:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var job analysisJobDTO

	err = tx.QueryRow(
		r.Context(),
		`
		INSERT INTO analysis_jobs (image_id, model_id, status, started_at, finished_at)
		VALUES ($1, $2, 'finished', NOW(), NOW())
		RETURNING id, image_id, model_id, status, created_at, COALESCE(finished_at::text, '')
		`,
		imageID,
		modelID,
	).Scan(
		&job.ID,
		&job.ImageID,
		&job.ModelID,
		&job.Status,
		&job.CreatedAt,
		&job.FinishedAt,
	)

	if err != nil {
		log.Println("insert analysis job:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rows, err := tx.Query(
		r.Context(),
		`
		INSERT INTO analysis_results (job_id, image_id, label, confidence, x, y, width, height)
		VALUES
			($1, $2, 'suspicious_area', 0.8700, 120, 90, 180, 120),
			($1, $2, 'suspicious_area', 0.7600, 360, 160, 140, 110)
		RETURNING id, job_id, image_id, label, confidence::float8, x, y, width, height, created_at
		`,
		job.ID,
		imageID,
	)
	if err != nil {
		log.Println("insert analysis results:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	results := make([]analysisResultDTO, 0)

	for rows.Next() {
		var result analysisResultDTO

		if err := rows.Scan(
			&result.ID,
			&result.JobID,
			&result.ImageID,
			&result.Label,
			&result.Confidence,
			&result.X,
			&result.Y,
			&result.Width,
			&result.Height,
			&result.CreatedAt,
		); err != nil {
			rows.Close()
			log.Println("scan analysis result:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		rows.Close()
		log.Println("rows analysis results:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	rows.Close()

	if _, err := tx.Exec(
		r.Context(),
		`
		UPDATE dental_images
		SET status = 'analyzed'
		WHERE id = $1
		`,
		imageID,
	); err != nil {
		log.Println("update image status:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Println("commit analysis tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	job.Results = results

	writeJSON(w, http.StatusCreated, job)
}

func (a *App) GetImageAnalysis(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	imageID, err := strconv.ParseInt(chi.URLParam(r, "imageID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid image id")
		return
	}

	if !a.imageBelongsToDoctor(r, imageID, claims.UserID) {
		writeError(w, http.StatusNotFound, "image not found")
		return
	}

	rows, err := a.db.Query(
		r.Context(),
		`
		SELECT id, job_id, image_id, label, confidence::float8, x, y, width, height, created_at
		FROM analysis_results
		WHERE image_id = $1
		ORDER BY created_at DESC
		`,
		imageID,
	)
	if err != nil {
		log.Println("get analysis:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	results := make([]analysisResultDTO, 0)

	for rows.Next() {
		var result analysisResultDTO

		if err := rows.Scan(
			&result.ID,
			&result.JobID,
			&result.ImageID,
			&result.Label,
			&result.Confidence,
			&result.X,
			&result.Y,
			&result.Width,
			&result.Height,
			&result.CreatedAt,
		); err != nil {
			log.Println("scan analysis:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		log.Println("rows analysis:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"image_id": imageID,
		"results":  results,
	})
}
