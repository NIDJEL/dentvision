package integration

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/NIDJEL/dentvision/backend/internal/server"
	"github.com/NIDJEL/dentvision/backend/internal/testutil"
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
	db := testutil.MustConnectDB(t)
	defer db.Close()

	testutil.CleanupDB(t, db)
	testutil.EnsureBaseData(t, db)

	app := server.New(db, "test_secret", t.TempDir())

	ts := httptest.NewServer(app.Routes())
	defer ts.Close()

	client := ts.Client()

	status, body := testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/health", "", nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	var login loginResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+"/auth/login",
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

	status, body = testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/me", login.Token, nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	var patient patientResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+"/patients",
		login.Token,
		map[string]string{
			"full_name":  "Иванов Иван Иванович",
			"birth_date": "1995-04-12",
			"phone":      "+79990000000",
			"comment":    "Тестовый пациент",
		},
		&patient,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if patient.ID == 0 {
		t.Fatal("expected patient id")
	}

	status, body = testutil.DoJSON(t, client, http.MethodGet, ts.URL+"/patients", login.Token, nil, nil)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	var image imageResponse

	status, body = testutil.DoImageUpload(
		t,
		client,
		ts.URL+"/patients/1/images",
		login.Token,
		&image,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if image.ID == 0 {
		t.Fatal("expected image id")
	}

	var analysis analysisResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodPost,
		ts.URL+"/images/1/analysis",
		login.Token,
		nil,
		&analysis,
	)
	testutil.RequireStatus(t, status, http.StatusCreated, body)

	if len(analysis.Results) == 0 {
		t.Fatal("expected analysis results")
	}

	var savedAnalysis getAnalysisResponse

	status, body = testutil.DoJSON(
		t,
		client,
		http.MethodGet,
		ts.URL+"/images/1/analysis",
		login.Token,
		nil,
		&savedAnalysis,
	)
	testutil.RequireStatus(t, status, http.StatusOK, body)

	if len(savedAnalysis.Results) == 0 {
		t.Fatal("expected saved analysis results")
	}
}
