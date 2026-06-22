package server

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
)

type createPatientRequest struct {
	FullName  string `json:"full_name"`
	BirthDate string `json:"birth_date"`
	Phone     string `json:"phone"`
	Comment   string `json:"comment"`
}

type patientDTO struct {
	ID        int64     `json:"id"`
	DoctorID  int64     `json:"doctor_id"`
	FullName  string    `json:"full_name"`
	BirthDate string    `json:"birth_date"`
	Phone     string    `json:"phone"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

func (a *App) CreatePatient(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req createPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	req.BirthDate = strings.TrimSpace(req.BirthDate)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "full_name is required")
		return
	}

	var patient patientDTO

	err := a.db.QueryRow(
		r.Context(),
		`
		INSERT INTO patients (doctor_id, full_name, birth_date, phone, comment)
		VALUES ($1, $2, NULLIF($3, '')::date, NULLIF($4, ''), NULLIF($5, ''))
		RETURNING id, doctor_id, full_name, COALESCE(birth_date::text, ''), COALESCE(phone, ''), COALESCE(comment, ''), created_at
		`,
		claims.UserID,
		req.FullName,
		req.BirthDate,
		req.Phone,
		req.Comment,
	).Scan(
		&patient.ID,
		&patient.DoctorID,
		&patient.FullName,
		&patient.BirthDate,
		&patient.Phone,
		&patient.Comment,
		&patient.CreatedAt,
	)

	if err != nil {
		log.Println("create patient:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusCreated, patient)
}

func (a *App) ListPatients(w http.ResponseWriter, r *http.Request) {
	claims, ok := currentUserClaims(r)
	if !ok {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	rows, err := a.db.Query(
		r.Context(),
		`
		SELECT id, doctor_id, full_name, COALESCE(birth_date::text, ''), COALESCE(phone, ''), COALESCE(comment, ''), created_at
		FROM patients
		WHERE doctor_id = $1
		ORDER BY created_at DESC
		`,
		claims.UserID,
	)
	if err != nil {
		log.Println("list patients:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer rows.Close()

	patients := make([]patientDTO, 0)

	for rows.Next() {
		var patient patientDTO

		if err := rows.Scan(
			&patient.ID,
			&patient.DoctorID,
			&patient.FullName,
			&patient.BirthDate,
			&patient.Phone,
			&patient.Comment,
			&patient.CreatedAt,
		); err != nil {
			log.Println("scan patient:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		patients = append(patients, patient)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"patients": patients,
	})
}

func (a *App) GetPatient(w http.ResponseWriter, r *http.Request) {
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

	var patient patientDTO

	err = a.db.QueryRow(
		r.Context(),
		`
		SELECT id, doctor_id, full_name, COALESCE(birth_date::text, ''), COALESCE(phone, ''), COALESCE(comment, ''), created_at
		FROM patients
		WHERE id = $1 AND doctor_id = $2
		LIMIT 1
		`,
		patientID,
		claims.UserID,
	).Scan(
		&patient.ID,
		&patient.DoctorID,
		&patient.FullName,
		&patient.BirthDate,
		&patient.Phone,
		&patient.Comment,
		&patient.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "patient not found")
			return
		}

		log.Println("get patient:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, patient)
}

func (a *App) UpdatePatient(w http.ResponseWriter, r *http.Request) {
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

	var req createPatientRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid json")
		return
	}

	req.FullName = strings.TrimSpace(req.FullName)
	req.BirthDate = strings.TrimSpace(req.BirthDate)
	req.Phone = strings.TrimSpace(req.Phone)
	req.Comment = strings.TrimSpace(req.Comment)

	if req.FullName == "" {
		writeError(w, http.StatusBadRequest, "full_name is required")
		return
	}

	var patient patientDTO

	err = a.db.QueryRow(
		r.Context(),
		`
		UPDATE patients
		SET full_name = $3,
			birth_date = NULLIF($4, '')::date,
			phone = NULLIF($5, ''),
			comment = NULLIF($6, ''),
			updated_at = NOW()
		WHERE id = $1 AND doctor_id = $2
		RETURNING id, doctor_id, full_name, COALESCE(birth_date::text, ''), COALESCE(phone, ''), COALESCE(comment, ''), created_at
		`,
		patientID,
		claims.UserID,
		req.FullName,
		req.BirthDate,
		req.Phone,
		req.Comment,
	).Scan(
		&patient.ID,
		&patient.DoctorID,
		&patient.FullName,
		&patient.BirthDate,
		&patient.Phone,
		&patient.Comment,
		&patient.CreatedAt,
	)

	if err != nil {
		if err == pgx.ErrNoRows {
			writeError(w, http.StatusNotFound, "patient not found")
			return
		}

		log.Println("update patient:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	writeJSON(w, http.StatusOK, patient)
}

func (a *App) DeletePatient(w http.ResponseWriter, r *http.Request) {
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

	tx, err := a.db.Begin(r.Context())
	if err != nil {
		log.Println("begin delete patient tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}
	defer tx.Rollback(r.Context())

	rows, err := tx.Query(
		r.Context(),
		`
		SELECT f.id, f.file_path
		FROM dental_images di
		JOIN image_files f ON f.id = di.file_id
		JOIN patients p ON p.id = di.patient_id
		WHERE p.id = $1 AND p.doctor_id = $2
		`,
		patientID,
		claims.UserID,
	)
	if err != nil {
		log.Println("select patient files:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	type patientFile struct {
		id   int64
		path string
	}

	files := make([]patientFile, 0)
	for rows.Next() {
		var file patientFile
		if err := rows.Scan(&file.id, &file.path); err != nil {
			rows.Close()
			log.Println("scan patient file:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}

		files = append(files, file)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		log.Println("rows patient files:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	tag, err := tx.Exec(
		r.Context(),
		`
		DELETE FROM patients
		WHERE id = $1 AND doctor_id = $2
		`,
		patientID,
		claims.UserID,
	)
	if err != nil {
		log.Println("delete patient:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	if tag.RowsAffected() == 0 {
		writeError(w, http.StatusNotFound, "patient not found")
		return
	}

	for _, file := range files {
		if _, err := tx.Exec(
			r.Context(),
			`
			DELETE FROM image_files
			WHERE id = $1
			`,
			file.id,
		); err != nil {
			log.Println("delete patient image file row:", err)
			writeError(w, http.StatusInternalServerError, "internal server error")
			return
		}
	}

	if err := tx.Commit(r.Context()); err != nil {
		log.Println("commit delete patient tx:", err)
		writeError(w, http.StatusInternalServerError, "internal server error")
		return
	}

	for _, file := range files {
		if err := os.Remove(file.path); err != nil && !os.IsNotExist(err) {
			log.Println("remove patient image file:", err)
		}
	}

	w.WriteHeader(http.StatusNoContent)
}
