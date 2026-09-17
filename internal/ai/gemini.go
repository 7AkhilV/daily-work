package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/7AkhilV/daily-work/internal/summary"
)

const DefaultGeminiModel = "gemini-3.5-flash"

// GeminiProvider calls the free Google AI Studio Gemini API.
type GeminiProvider struct {
	APIKey string
	Model  string
	Client *http.Client
}

func NewGemini(apiKey, model string) *GeminiProvider {
	if model == "" || strings.HasPrefix(model, "llama") {
		model = DefaultGeminiModel
	}
	return &GeminiProvider{
		APIKey: apiKey,
		Model:  model,
		Client: &http.Client{Timeout: 90 * time.Second},
	}
}

type geminiReq struct {
	Contents         []geminiContent        `json:"contents"`
	GenerationConfig map[string]interface{} `json:"generationConfig,omitempty"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResp struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (g *GeminiProvider) Summarize(ctx context.Context, input ActivityInput) ([]summary.WorkItem, error) {
	if strings.TrimSpace(g.APIKey) == "" {
		return nil, fmt.Errorf("Gemini API key missing.\n\nRun:\n\n  daily-work auth")
	}
	prompt := buildPrompt(input)
	url := fmt.Sprintf(
		"https://generativelanguage.googleapis.com/v1beta/models/%s:generateContent?key=%s",
		g.Model,
		g.APIKey,
	)

	body, err := json.Marshal(geminiReq{
		Contents: []geminiContent{{Parts: []geminiPart{{Text: prompt}}}},
		GenerationConfig: map[string]interface{}{
			"temperature":      0.2,
			"responseMimeType": "application/json",
		},
	})
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Gemini request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Gemini API error (%d): %s", resp.StatusCode, truncate(string(raw), 400))
	}

	var parsed geminiResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse Gemini response: %w", err)
	}
	if parsed.Error != nil {
		return nil, fmt.Errorf("Gemini: %s", parsed.Error.Message)
	}
	if len(parsed.Candidates) == 0 || len(parsed.Candidates[0].Content.Parts) == 0 {
		return nil, fmt.Errorf("Gemini returned empty response")
	}
	return parseWorkItems(parsed.Candidates[0].Content.Parts[0].Text)
}
