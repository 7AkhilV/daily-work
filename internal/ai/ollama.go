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

const (
	DefaultOllamaHost  = "http://localhost:11434"
	DefaultOllamaModel = "llama3.2:3b"
)

// OllamaProvider talks to a local Ollama server (no API key).
type OllamaProvider struct {
	Host   string
	Model  string
	Client *http.Client
}

func NewOllama(host, model string) *OllamaProvider {
	if host == "" {
		host = DefaultOllamaHost
	}
	if model == "" {
		model = DefaultOllamaModel
	}
	return &OllamaProvider{
		Host:  strings.TrimRight(host, "/"),
		Model: model,
		Client: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

type ollamaChatReq struct {
	Model    string              `json:"model"`
	Messages []ollamaChatMessage `json:"messages"`
	Stream   bool                `json:"stream"`
	Format   string              `json:"format,omitempty"`
	Options  map[string]any      `json:"options,omitempty"`
}

type ollamaChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ollamaChatResp struct {
	Message ollamaChatMessage `json:"message"`
	Error   string            `json:"error"`
}

type ollamaTagsResp struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// Ping checks whether Ollama is reachable.
func (o *OllamaProvider) Ping(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Host+"/api/tags", nil)
	if err != nil {
		return err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return fmt.Errorf("Ollama is not running at %s\n\nInstall (macOS):\n  brew install ollama\n  ollama serve\n  ollama pull %s\n\nDetails: %w", o.Host, o.Model, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("Ollama returned HTTP %d", resp.StatusCode)
	}
	return nil
}

// HasModel reports whether the configured model is already downloaded.
func (o *OllamaProvider) HasModel(ctx context.Context) (bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, o.Host+"/api/tags", nil)
	if err != nil {
		return false, err
	}
	resp, err := o.Client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var tags ollamaTagsResp
	if err := json.Unmarshal(raw, &tags); err != nil {
		return false, err
	}
	want := strings.ToLower(o.Model)
	wantBase, wantTag := splitModel(want)
	for _, m := range tags.Models {
		got := strings.ToLower(m.Name)
		if got == want {
			return true, nil
		}
		base, tag := splitModel(got)
		if base == wantBase && tag == wantTag {
			return true, nil
		}
	}
	return false, nil
}

func splitModel(name string) (base, tag string) {
	parts := strings.SplitN(name, ":", 2)
	if len(parts) == 1 {
		return parts[0], "latest"
	}
	return parts[0], parts[1]
}

// EnsureReady verifies Ollama is up and the small model is present.
func (o *OllamaProvider) EnsureReady(ctx context.Context) error {
	if err := o.Ping(ctx); err != nil {
		return err
	}
	ok, err := o.HasModel(ctx)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("model %q not found locally (~2GB download)\n\nRun once:\n  ollama pull %s", o.Model, o.Model)
	}
	return nil
}

func (o *OllamaProvider) Summarize(ctx context.Context, input ActivityInput) ([]summary.WorkItem, error) {
	if err := o.EnsureReady(ctx); err != nil {
		return nil, err
	}

	prompt := buildPrompt(input) // compact prompt for small local models
	body, _ := json.Marshal(ollamaChatReq{
		Model: o.Model,
		Messages: []ollamaChatMessage{
			{Role: "system", Content: "You output JSON only. No markdown."},
			{Role: "user", Content: prompt},
		},
		Stream: false,
		Format: "json",
		Options: map[string]any{
			"temperature": 0.2,
			"num_predict": 512, // keep output small — faster, less RAM
		},
	})

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, o.Host+"/api/chat", bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Ollama request failed: %w", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Ollama error (%d): %s", resp.StatusCode, truncate(string(raw), 300))
	}

	var parsed ollamaChatResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, fmt.Errorf("parse Ollama response: %w", err)
	}
	if parsed.Error != "" {
		return nil, fmt.Errorf("Ollama: %s", parsed.Error)
	}
	if strings.TrimSpace(parsed.Message.Content) == "" {
		return nil, fmt.Errorf("Ollama returned empty response")
	}
	return parseWorkItems(parsed.Message.Content)
}
