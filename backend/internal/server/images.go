package server

import (
	"crypto/rand"
	"encoding/hex"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type imageDTO struct {
	ID           int64     `json:"id"`
	PatientID    int64     `json:"patient_id"`
	FileID       int64     `json:"file_id"`
	UploadedBy   int64     `json:"uploaded_by"`
	FilePath     string    `json:"file_path"`
	OriginalName string    `json:"original_name"`
	MimeType     string    `json:"mime_type"`
	FileSize     int64     `json:"file_size"`
	Status       string    `json:"status"`
	CreatedAt    time.Time `json:"created_at"`
}

func (a *App) UploadPatientImage(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	patientID, err := strconv.ParseInt(chi.URLParam(r, "patientID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid patient id")
		return
	}

	if !a.patientBelongsToDoctor(r, patientID, claims.UserID) {
		writeError(w, http.StatusNotFound, "patient not found")
		return
	}

	if err := r.ParseMultipartForm(20 << 20); err != nil {
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		writeError(w, http.StatusBadRequest, "image file is required")
		return
	}
	defer file.Close()

	if header.Size <= 0 {
		writeError(w, http.StatusBadRequest, "empty file")
		return
	}

	if header.Size > 20<<20 {
		writeError(w, http.StatusBadRequest, "file is too large")
		return
	}

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if ext != ".jpg" && ext != ".jpeg" && ext != ".png" {
		writeError(w, http.StatusBadRequest, "only jpg, jpeg and png are allowed")
		return
	}

	if err := os.MkdirAll(a.uploadsDir, 0755); err != nil {
		log.Println("mkdir uploads:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	fileName := randomFileName(ext)
	filePath := filepath.Join(a.uploadsDir, fileName)

	dst, err := os.Create(filePath)
	if err != nil {
		log.Println("create upload file:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer dst.Close()

	written, err := io.Copy(dst, file)
	if err != nil {
		log.Println("save upload file:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	mimeType := header.Header.Get("Content-Type")
	if mimeType == "" {
		mimeType = "application/octet-stream"
	}

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		log.Println("begin upload tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(r.Context())

	var fileID int64

	err = tx.QueryRow(
		r.Context(),
		`
		INSERT INTO image_files (file_path, original_name, mime_type, file_size)
		VALUES ($1, $2, $3, $4)
		RETURNING id
		`,
		filePath,
		header.Filename,
		mimeType,
		written,
	).Scan(&fileID)

	if err != nil {
		log.Println("insert image file:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	var image imageDTO

	err = tx.QueryRow(
		r.Context(),
		`
		INSERT INTO dental_images (patient_id, file_id, uploaded_by, image_type, status)
		VALUES ($1, $2, $3, 'xray', 'uploaded')
		RETURNING id, patient_id, file_id, uploaded_by, status, created_at
		`,
		patientID,
		fileID,
		claims.UserID,
	).Scan(
		&image.ID,
		&image.PatientID,
		&image.FileID,
		&image.UploadedBy,
		&image.Status,
		&image.CreatedAt,
	)

	if err != nil {
		log.Println("insert dental image:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Println("commit upload tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	image.FilePath = filePath
	image.OriginalName = header.Filename
	image.MimeType = mimeType
	image.FileSize = written

	writeJSON(w, http.StatusCreated, image)
}

func (a *App) ListPatientImages(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	patientID, err := strconv.ParseInt(chi.URLParam(r, "patientID"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid patient id")
		return
	}

	if !a.patientBelongsToDoctor(r, patientID, claims.UserID) {
		writeError(w, http.StatusNotFound, "patient not found")
		return
	}

	rows, err := a.db.Query(
		r.Context(),
		`
		SELECT
			di.id,
			di.patient_id,
			di.file_id,
			di.uploaded_by,
			f.file_path,
			f.original_name,
			f.mime_type,
			COALESCE(f.file_size, 0),
			di.status,
			di.created_at
		FROM dental_images di
		JOIN image_files f ON f.id = di.file_id
		WHERE di.patient_id = $1
		ORDER BY di.created_at DESC
		`,
		patientID,
	)
	if err != nil {
		log.Println("list images:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	images := make([]imageDTO, 0)

	for rows.Next() {
		var image imageDTO

		if err := rows.Scan(
			&image.ID,
			&image.PatientID,
			&image.FileID,
			&image.UploadedBy,
			&image.FilePath,
			&image.OriginalName,
			&image.MimeType,
			&image.FileSize,
			&image.Status,
			&image.CreatedAt,
		); err != nil {
			log.Println("scan image:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		images = append(images, image)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"images": images,
	})
}

func (a *App) patientBelongsToDoctor(r *http.Request, patientID int64, doctorID int64) bool {
	var exists bool

	err := a.db.QueryRow(
		r.Context(),
		`
		SELECT EXISTS (
			SELECT 1
			FROM patients
			WHERE id = $1 AND doctor_id = $2
		)
		`,
		patientID,
		doctorID,
	).Scan(&exists)

	return err == nil && exists
}

func (a *App) imageBelongsToDoctor(r *http.Request, imageID int64, doctorID int64) bool {
	var exists bool

	err := a.db.QueryRow(
		r.Context(),
		`
		SELECT EXISTS (
			SELECT 1
			FROM dental_images di
			JOIN patients p ON p.id = di.patient_id
			WHERE di.id = $1 AND p.doctor_id = $2
		)
		`,
		imageID,
		doctorID,
	).Scan(&exists)

	return err == nil && exists
}

func randomFileName(ext string) string {
	bytes := make([]byte, 16)

	if _, err := rand.Read(bytes); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 10) + ext
	}

	return hex.EncodeToString(bytes) + ext
}
