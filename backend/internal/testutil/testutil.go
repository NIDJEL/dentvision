package testutil

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

func MustConnectDB(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	db, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect test db: %v", err)
	}

	if err := db.Ping(ctx); err != nil {
		t.Fatalf("ping test db: %v", err)
	}

	return db
}

func CleanupDB(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := db.Exec(
		ctx,
		`
		TRUNCATE TABLE
			doctor_feedback,
			analysis_results,
			analysis_jobs,
			dental_images,
			image_files,
			patients
		RESTART IDENTITY CASCADE
		`,
	)
	if err != nil {
		t.Fatalf("cleanup db: %v", err)
	}
}

func EnsureBaseData(t *testing.T, db *pgxpool.Pool) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	passwordHash, err := bcrypt.GenerateFromPassword([]byte("doctor123"), bcrypt.DefaultCost)
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}

	_, err = db.Exec(
		ctx,
		`
		INSERT INTO roles (name, title)
		VALUES ('doctor', 'Врач')
		ON CONFLICT (name) DO NOTHING
		`,
	)
	if err != nil {
		t.Fatalf("insert doctor role: %v", err)
	}

	_, err = db.Exec(
		ctx,
		`
		INSERT INTO users (role_id, email, password_hash, full_name, is_active)
		VALUES (
			(SELECT id FROM roles WHERE name = 'doctor'),
			$1,
			$2,
			$3,
			TRUE
		)
		ON CONFLICT (email)
		DO UPDATE SET
			password_hash = EXCLUDED.password_hash,
			full_name = EXCLUDED.full_name,
			is_active = TRUE
		`,
		"doctor@dentvision.com",
		string(passwordHash),
		"Тестовый врач",
	)
	if err != nil {
		t.Fatalf("insert test doctor: %v", err)
	}

	_, err = db.Exec(
		ctx,
		`
		INSERT INTO analysis_models (name, version, description, is_active)
		SELECT
			'DentVision Demo Model',
			'0.1.0',
			'Тестовая модель для предварительного анализа стоматологических снимков',
			TRUE
		WHERE NOT EXISTS (
			SELECT 1
			FROM analysis_models
			WHERE name = 'DentVision Demo Model'
			  AND version = '0.1.0'
		)
		`,
	)
	if err != nil {
		t.Fatalf("insert analysis model: %v", err)
	}
}

func DoJSON(t *testing.T, client *http.Client, method string, url string, token string, payload any, out any) (int, []byte) {
	t.Helper()

	var body io.Reader

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			t.Fatalf("marshal json: %v", err)
		}

		body = bytes.NewReader(data)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}

	req.Header.Set("Content-Type", "application/json")

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response body: %v", err)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			t.Fatalf("decode json response: %v\nbody: %s", err, string(respBody))
		}
	}

	return resp.StatusCode, respBody
}

func DoImageUpload(t *testing.T, client *http.Client, url string, token string, out any) (int, []byte) {
	t.Helper()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("image", "test-image.png")
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}

	testPNG := []byte{
		0x89, 0x50, 0x4E, 0x47,
		0x0D, 0x0A, 0x1A, 0x0A,
		0x00, 0x00, 0x00, 0x0D,
		0x49, 0x48, 0x44, 0x52,
	}

	if _, err := part.Write(testPNG); err != nil {
		t.Fatalf("write test image: %v", err)
	}

	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart writer: %v", err)
	}

	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		t.Fatalf("new upload request: %v", err)
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("do upload request: %v", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read upload response: %v", err)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			t.Fatalf("decode upload response: %v\nbody: %s", err, string(respBody))
		}
	}

	return resp.StatusCode, respBody
}

func RequireStatus(t *testing.T, got int, want int, body []byte) {
	t.Helper()

	if got != want {
		t.Fatalf("expected status %d, got %d\nbody: %s", want, got, string(body))
	}
}
