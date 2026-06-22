package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const mlAnalysisTimeout = 15 * time.Second

type mlAnalyzeRequest struct {
	ImagePath string `json:"image_path"`
}

type mlAnalyzeResponse struct {
	Results []mlAnalysisResult `json:"results"`
}

type mlAnalysisResult struct {
	Label      string  `json:"label"`
	Confidence float64 `json:"confidence"`
	X          int     `json:"x"`
	Y          int     `json:"y"`
	Width      int     `json:"width"`
	Height     int     `json:"height"`
}

func (a *App) requestMLAnalysis(ctx context.Context, imagePath string) ([]mlAnalysisResult, error) {
	body, err := json.Marshal(mlAnalyzeRequest{
		ImagePath: imagePath,
	})
	if err != nil {
		return nil, fmt.Errorf("prepare ml request: %w", err)
	}

	ctx, cancel := context.WithTimeout(ctx, mlAnalysisTimeout)
	defer cancel()

	endpoint := strings.TrimRight(a.mlServiceURL, "/") + "/analyze"

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("create ml request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("call ml service: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		responseBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		message := strings.TrimSpace(string(responseBody))
		if message == "" {
			message = http.StatusText(resp.StatusCode)
		}

		return nil, fmt.Errorf("ml service returned status %d: %s", resp.StatusCode, message)
	}

	var mlResponse mlAnalyzeResponse
	if err := json.NewDecoder(resp.Body).Decode(&mlResponse); err != nil {
		return nil, fmt.Errorf("decode ml response: %w", err)
	}

	if err := validateMLResults(mlResponse.Results); err != nil {
		return nil, err
	}

	return mlResponse.Results, nil
}

func validateMLResults(results []mlAnalysisResult) error {
	for index, result := range results {
		if strings.TrimSpace(result.Label) == "" {
			return fmt.Errorf("ml service returned invalid result %d: label is required", index)
		}

		if result.Confidence < 0 || result.Confidence > 1 {
			return fmt.Errorf("ml service returned invalid result %d: confidence is out of range", index)
		}

		if result.X < 0 || result.Y < 0 || result.Width <= 0 || result.Height <= 0 {
			return fmt.Errorf("ml service returned invalid result %d: invalid bounding box", index)
		}
	}

	return nil
}

func analysisErrorMessage(err error) string {
	message := strings.TrimSpace(err.Error())
	if len(message) > 300 {
		message = message[:300]
	}

	return message
}
