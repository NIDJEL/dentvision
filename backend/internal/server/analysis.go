package server

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
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

type analysisStore interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
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

	ctx := r.Context()

	tx, err := a.db.Begin(ctx)
	if err != nil {
		log.Println("begin analysis tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(ctx)

	var imagePath string

	err = tx.QueryRow(
		ctx,
		`
		SELECT f.file_path
		FROM dental_images di
		JOIN patients p ON p.id = di.patient_id
		JOIN image_files f ON f.id = di.file_id
		WHERE di.id = $1 AND p.doctor_id = $2
		FOR UPDATE OF di
		`,
		imageID,
		claims.UserID,
	).Scan(&imagePath)
	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "image not found")
			return
		}

		log.Println("lock image for analysis:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	existingJob, found, err := a.getLatestFinishedAnalysisJob(ctx, tx, imageID)
	if err != nil {
		log.Println("get latest finished analysis job:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if found {
		if err := tx.Commit(ctx); err != nil {
			log.Println("commit cached analysis tx:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		writeJSON(w, http.StatusOK, existingJob)
		return
	}

	var modelID int64

	err = tx.QueryRow(
		ctx,
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
		ctx,
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

	mlResults, err := a.requestMLAnalysis(ctx, imagePath)
	if err != nil {
		message := analysisErrorMessage(err)
		if updateErr := a.markAnalysisJobFailed(ctx, tx, job.ID, message); updateErr != nil {
			log.Println("mark analysis job failed:", updateErr)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		if commitErr := tx.Commit(ctx); commitErr != nil {
			log.Println("commit failed analysis job:", commitErr)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		log.Println("ml analysis:", err)
		writeError(w, http.StatusBadGateway, "ml service analysis failed: "+message)
		return
	}

	results := make([]analysisResultDTO, 0, len(mlResults))

	for _, mlResult := range mlResults {
		var result analysisResultDTO

		err = tx.QueryRow(
			ctx,
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
			log.Println("scan analysis result:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		results = append(results, result)
	}

	if _, err := tx.Exec(
		ctx,
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

	err = tx.QueryRow(
		ctx,
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
		log.Println("finish analysis job:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(ctx); err != nil {
		log.Println("commit analysis tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	job.Results = results

	writeJSON(w, http.StatusCreated, job)
}

func (a *App) getLatestFinishedAnalysisJob(ctx context.Context, store analysisStore, imageID int64) (analysisJobDTO, bool, error) {
	var job analysisJobDTO

	err := store.QueryRow(
		ctx,
		`
		SELECT
			id,
			image_id,
			model_id,
			status,
			COALESCE(error_message, ''),
			created_at,
			COALESCE(finished_at::text, '')
		FROM analysis_jobs
		WHERE image_id = $1 AND status = 'finished'
		ORDER BY finished_at DESC, id DESC
		LIMIT 1
		`,
		imageID,
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
		if err == pgx.ErrNoRows {
			return analysisJobDTO{}, false, nil
		}

		return analysisJobDTO{}, false, err
	}

	results, err := a.getAnalysisResultsByJob(ctx, store, job.ID)
	if err != nil {
		return analysisJobDTO{}, false, err
	}

	job.Results = results

	return job, true, nil
}

func (a *App) getAnalysisResultsByJob(ctx context.Context, store analysisStore, jobID int64) ([]analysisResultDTO, error) {
	rows, err := store.Query(
		ctx,
		`
		SELECT id, job_id, image_id, label, confidence::float8, x, y, width, height, created_at
		FROM analysis_results
		WHERE job_id = $1
		ORDER BY id ASC
		`,
		jobID,
	)
	if err != nil {
		return nil, err
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
			return nil, err
		}

		results = append(results, result)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return results, nil
}

func (a *App) markAnalysisJobFailed(ctx context.Context, store analysisStore, jobID int64, message string) error {
	_, err := store.Exec(
		ctx,
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

	job, found, err := a.getLatestFinishedAnalysisJob(r.Context(), a.db, imageID)
	if err != nil {
		log.Println("get analysis:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	results := make([]analysisResultDTO, 0)
	if found {
		results = job.Results
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"image_id": imageID,
		"results":  results,
	})
}
