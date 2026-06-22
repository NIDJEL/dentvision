package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NIDJEL/dentvision/backend/internal/server"
	"github.com/NIDJEL/dentvision/backend/internal/testutil"
	"github.com/jackc/pgx/v5/pgxpool"
)

type loginResponse struct {
	Token string `json:"token"`
	User  struct {
		ID       int64  `json:"id"`
		Email    string `json:"email"`
		FullName string `json:"full_name"`
		Role     string `json:"role"`
	} `json:"user"`
}

type patientResponse struct {
	ID       int64  `json:"id"`
	FullName string `json:"full_name"`
}

type imageResponse struct {
	ID           int64  `json:"id"`
	PatientID    int64  `json:"patient_id"`
	OriginalName string `json:"original_name"`
	Status       string `json:"status"`
}

type analysisResponse struct {
	ID      int64  `json:"id"`
	ImageID int64  `json:"image_id"`
	Status  string `json:"status"`
	Results []struct {
		ID         int64   `json:"id"`
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
		X          int     `json:"x"`
		Y          int     `json:"y"`
		Width      int     `json:"width"`
		Height     int     `json:"height"`
	} `json:"results"`
}

type getAnalysisResponse struct {
	ImageID int64 `json:"image_id"`
	Results []struct {
		ID         int64   `json:"id"`
		Label      string  `json:"label"`
		Confidence float64 `json:"confidence"`
	} `json:"results"`
}

func TestDoctorMainFlow(t *testing.T) {
	mlService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/analyze" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		var req struct {
			ImagePath string `json:"image_path"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid json", http.StatusBadRequest)
			return
		}

		if req.ImagePath == "" {
			http.Error(w, "image_path is required", http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"results": [
				{
					"label": "mock_finding",
					"confidence": 0.91,
					"x": 11,
					"y": 22,
					"width": 33,
					"height": 44
				}
			]
		}`))
	}))
	defer mlService.Close()

	_, ts, client := startBackendWithML(t, mlService.URL)

	status, body := testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/health", "", nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	login := loginDoctor(t, client, ts.URL)

	status, body = testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/me", login.Token, nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	patient := createTestPatient(t, client, ts.URL, login.Token)

	status, body = testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/patients", login.Token, nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	image := uploadTestImage(t, client, ts.URL, login.Token, patient.ID)

	var analysis analysisResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		fmt.Sprintf("%s/images/%d/analysis", ts.URL, image.ID),
		login.Token,
		nil,
		&analysis,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if analysis.Status != "finished" {
		t.Fatalf("expected finished analysis job, got %q", analysis.Status)
	}

	if len(analysis.Results) != 1 {
		t.Fatalf("expected one analysis result, got %d", len(analysis.Results))
	}

	result := analysis.Results[0]

	if result.Label != "mock_finding" {
		t.Fatalf("expected ML label mock_finding, got %q", result.Label)
	}

	if result.Confidence != 0.91 {
		t.Fatalf("expected ML confidence 0.91, got %v", result.Confidence)
	}

	if result.X != 11 || result.Y != 22 || result.Width != 33 || result.Height != 44 {
		t.Fatalf(
			"expected ML box 11,22,33,44; got %d,%d,%d,%d",
			result.X,
			result.Y,
			result.Width,
			result.Height,
		)
	}

	var savedAnalysis getAnalysisResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodGet,
		fmt.Sprintf("%s/images/%d/analysis", ts.URL, image.ID),
		login.Token,
		nil,
		&savedAnalysis,
	)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	if len(savedAnalysis.Results) != 1 {
		t.Fatalf("expected one saved analysis result, got %d", len(savedAnalysis.Results))
	}

	if got := savedAnalysis.Results[0].Label; got != "mock_finding" {
		t.Fatalf("expected saved ML label mock_finding, got %q", got)
	}
}

func TestRunImageAnalysisMarksJobFailedWhenMLServiceFails(t *testing.T) {
	mlService := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "ml unavailable", http.StatusServiceUnavailable)
	}))
	defer mlService.Close()

	db, ts, client := startBackendWithML(t, mlService.URL)
	login := loginDoctor(t, client, ts.URL)
	patient := createTestPatient(t, client, ts.URL, login.Token)
	image := uploadTestImage(t, client, ts.URL, login.Token, patient.ID)

	status, body := testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		fmt.Sprintf("%s/images/%d/analysis", ts.URL, image.ID),
		login.Token,
		nil,
		nil,
	)
	testutil.RequireStatus(t, status, http.StatusBadGateway, body)

	var jobStatus string
	var errorMessage string

	err := db.QueryRow(
		context.Background(),
		`
		SELECT status, COALESCE(error_message, '')
		FROM analysis_jobs
		WHERE image_id = $1
		ORDER BY id DESC
		LIMIT 1
		`,
		image.ID,
	).Scan(&jobStatus, &errorMessage)
	if err != nil {
		t.Fatalf("query failed analysis job: %v", err)
	}

	if jobStatus != "failed" {
		t.Fatalf("expected failed job status, got %q", jobStatus)
	}

	if errorMessage == "" {
		t.Fatal("expected failed job error_message")
	}

	var imageStatus string

	err = db.QueryRow(
		context.Background(),
		`
		SELECT status
		FROM dental_images
		WHERE id = $1
		`,
		image.ID,
	).Scan(&imageStatus)
	if err != nil {
		t.Fatalf("query image status: %v", err)
	}

	if imageStatus != "uploaded" {
		t.Fatalf("expected image status to remain uploaded, got %q", imageStatus)
	}

	var resultCount int

	err = db.QueryRow(
		context.Background(),
		`
		SELECT COUNT(*)
		FROM analysis_results
		WHERE image_id = $1
		`,
		image.ID,
	).Scan(&resultCount)
	if err != nil {
		t.Fatalf("query analysis result count: %v", err)
	}

	if resultCount != 0 {
		t.Fatalf("expected no saved analysis results, got %d", resultCount)
	}
}

func startBackendWithML(t *testing.T, mlServiceURL string) (*pgxpool.Pool, *httptest.Server, *http.Client) {
	t.Helper()

	db := testutil.MustConnectDB(t)
	t.Cleanup(db.Close)

	testutil.CleanupDB(t, db)
	testutil.EnsureBaseData(t, db)

	app := server.New(db, "test_secret", t.TempDir(), mlServiceURL)

	ts := httptest.NewServer(app.Routes())
	t.Cleanup(ts.Close)

	return db, ts, ts.Client()
}

func loginDoctor(t *testing.T, client *http.Client, baseURL string) loginResponse {
	t.Helper()

	var login loginResponse

	status, body := testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		baseURL+"/auth/login",
		"",
		map[string]string{
			"email":    "doctor@dentvision.com",
			"password": "doctor123",
		},
		&login,
	)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	if login.Token == "" {
		t.Fatal("expected login token")
	}

	return login
}

func createTestPatient(t *testing.T, client *http.Client, baseURL string, token string) patientResponse {
	t.Helper()

	var patient patientResponse

	status, body := testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		baseURL+"/patients",
		token,
		map[string]string{
			"full_name":  "Ivan Ivanov",
			"birth_date": "1995-04-12",
			"phone":      "+79990000000",
			"comment":    "Integration test patient",
		},
		&patient,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if patient.ID == 0 {
		t.Fatal("expected patient id")
	}

	return patient
}

func uploadTestImage(t *testing.T, client *http.Client, baseURL string, token string, patientID int64) imageResponse {
	t.Helper()

	var image imageResponse

	status, body := testutil.DoImageUpload(
		t,
		client,
		fmt.Sprintf("%s/patients/%d/images", baseURL, patientID),
		token,
		&image,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if image.ID == 0 {
		t.Fatal("expected image id")
	}

	return image
}
