package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type ASRRequest struct {
	AudioPath string
	Language  string
}

type ASRResult struct {
	Text string
}

type ASRClient interface {
	Provider() string
	Model() string
	Transcribe(ctx context.Context, req ASRRequest) (ASRResult, error)
}

type OpenAIASRClient struct {
	baseURL    string
	apiKey     string
	model      string
	httpClient *http.Client
}

func NewOpenAIASRClient(baseURL string, apiKey string, model string) *OpenAIASRClient {
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	if model == "" {
		model = DefaultASRModel
	}
	return &OpenAIASRClient{
		baseURL: strings.TrimRight(baseURL, "/"),
		apiKey:  apiKey,
		model:   model,
		httpClient: &http.Client{
			Timeout: 10 * time.Minute,
		},
	}
}

func (c *OpenAIASRClient) Provider() string {
	return DefaultASRProvider
}

func (c *OpenAIASRClient) Model() string {
	return c.model
}

func (c *OpenAIASRClient) Transcribe(ctx context.Context, req ASRRequest) (ASRResult, error) {
	audioFile, err := os.Open(req.AudioPath)
	if err != nil {
		return ASRResult{}, err
	}
	defer audioFile.Close()

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filepath.Base(req.AudioPath))
	if err != nil {
		return ASRResult{}, err
	}
	if _, err := io.Copy(part, audioFile); err != nil {
		return ASRResult{}, err
	}
	if err := writer.WriteField("model", c.model); err != nil {
		return ASRResult{}, err
	}
	if err := writer.WriteField("response_format", "json"); err != nil {
		return ASRResult{}, err
	}
	if req.Language != "" {
		if err := writer.WriteField("language", req.Language); err != nil {
			return ASRResult{}, err
		}
	}
	if err := writer.Close(); err != nil {
		return ASRResult{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/audio/transcriptions", &body)
	if err != nil {
		return ASRResult{}, err
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return ASRResult{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return ASRResult{}, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ASRResult{}, fmt.Errorf("asr request failed: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return ASRResult{}, fmt.Errorf("parse asr response: %w", err)
	}
	if strings.TrimSpace(parsed.Text) == "" {
		return ASRResult{}, fmt.Errorf("asr response text is empty")
	}
	return ASRResult{Text: parsed.Text}, nil
}
