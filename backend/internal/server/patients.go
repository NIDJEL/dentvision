package server

import (
	"encoding/json"
	"log"
	"net/http"
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
