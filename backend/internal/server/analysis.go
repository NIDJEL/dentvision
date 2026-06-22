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
	ID           int64               `json:"id"`
	ImageID      int64               `json:"image_id"`
	ModelID      int64               `json:"model_id"`
	Status       string              `json:"status"`
	ErrorMessage string              `json:"error_message,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	FinishedAt   string              `json:"finished_at"`
	Results      []analysisResultDTO `json:"results"`
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

	var imagePath string

	err = a.db.QueryRow(
		r.Context(),
		`
		SELECT f.file_path
		FROM dental_images di
		JOIN image_files f ON f.id = di.file_id
		WHERE di.id = $1
		LIMIT 1
		`,
		imageID,
	).Scan(&imagePath)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}

		log.Println("select image file path:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var modelID int64

	err = a.db.QueryRow(
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

	err = a.db.QueryRow(
		r.Context(),
		`
		INSERT INTO analysis_jobs (image_id, model_id, status, started_at)
		VALUES ($1, $2, 'running', NOW())
		RETURNING
			id,
			image_id,
			model_id,
			status,
			COALESCE(error_message, ''),
			created_at,
			COALESCE(finished_at::text, '')
		`,
		imageID,
		modelID,
	).Scan(
		&job.ID,
		&job.ImageID,
		&job.ModelID,
		&job.Status,
		&job.ErrorMessage,
		&job.CreatedAt,
		&job.FinishedAt,
	)

	if err != nil {
		log.Println("insert analysis job:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	mlResults, err := a.requestMLAnalysis(r.Context(), imagePath)
	if err != nil {
		message := analysisErrorMessage(err)
		if updateErr := a.markAnalysisJobFailed(r, job.ID, message); updateErr != nil {
			log.Println("mark analysis job failed:", updateErr)
		}

		log.Println("ml analysis:", err)
		writeError(w, http.StatusBadGateway, "ml service analysis failed: "+message)
		return
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		log.Println("begin analysis tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	results := make([]analysisResultDTO, 0, len(mlResults))

	for _, mlResult := range mlResults {
		var result analysisResultDTO

		err = tx.QueryRow(
			r.Context(),
			`
			INSERT INTO analysis_results (job_id, image_id, label, confidence, x, y, width, height)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			RETURNING id, job_id, image_id, label, confidence::float8, x, y, width, height, created_at
			`,
			job.ID,
			imageID,
			mlResult.Label,
			mlResult.Confidence,
			mlResult.X,
			mlResult.Y,
			mlResult.Width,
			mlResult.Height,
		).Scan(
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
		)

		if err != nil {
			_ = tx.Rollback(r.Context())
			if updateErr := a.markAnalysisJobFailed(r, job.ID, "save analysis results failed"); updateErr != nil {
				log.Println("mark analysis job failed:", updateErr)
			}

			log.Println("scan analysis result:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		results = append(results, result)
	}

	if _, err := tx.Exec(
		r.Context(),
		`
		UPDATE dental_images
		SET status = 'analyzed'
		WHERE id = $1
		`,
		imageID,
	); err != nil {
		_ = tx.Rollback(r.Context())
		if updateErr := a.markAnalysisJobFailed(r, job.ID, "update image status failed"); updateErr != nil {
			log.Println("mark analysis job failed:", updateErr)
		}

		log.Println("update image status:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	err = tx.QueryRow(
		r.Context(),
		`
		UPDATE analysis_jobs
		SET status = 'finished',
			error_message = NULL,
			finished_at = NOW()
		WHERE id = $1
		RETURNING status, COALESCE(finished_at::text, '')
		`,
		job.ID,
	).Scan(&job.Status, &job.FinishedAt)

	if err != nil {
		_ = tx.Rollback(r.Context())
		if updateErr := a.markAnalysisJobFailed(r, job.ID, "finish analysis job failed"); updateErr != nil {
			log.Println("mark analysis job failed:", updateErr)
		}

		log.Println("finish analysis job:", err)
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

func (a *App) markAnalysisJobFailed(r *http.Request, jobID int64, message string) error {
	_, err := a.db.Exec(
		r.Context(),
		`
		UPDATE analysis_jobs
		SET status = 'failed',
			error_message = $2,
			finished_at = NOW()
		WHERE id = $1
		`,
		jobID,
		message,
	)

	return err
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
