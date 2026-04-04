package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/batkiz/rss-gateway/internal/config"
	"github.com/batkiz/rss-gateway/internal/model"
)

type OpenAIProcessor struct {
	baseURL string
	apiKey  string
	model   string
	client  *http.Client
}

type responsesRequest struct {
	Model        string             `json:"model"`
	Instructions string             `json:"instructions,omitempty"`
	Input        []responsesMessage `json:"input"`
}

type responsesMessage struct {
	Role    string                 `json:"role"`
	Content []responsesContentPart `json:"content"`
}

type responsesContentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type responsesAPIResponse struct {
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

type structuredResult struct {
	Title   string `json:"title"`
	Summary string `json:"summary"`
	Content string `json:"content"`
}

func NewOpenAIProcessor(cfg config.LLMConfig) (*OpenAIProcessor, error) {
	timeout, err := time.ParseDuration(cfg.Timeout)
	if err != nil {
		return nil, fmt.Errorf("parse llm timeout: %w", err)
	}
	return &OpenAIProcessor{
		baseURL: strings.TrimRight(cfg.BaseURL, "/"),
		apiKey:  cfg.APIKey,
		model:   cfg.Model,
		client:  &http.Client{Timeout: timeout},
	}, nil
}

func (p *OpenAIProcessor) Process(ctx context.Context, req model.ProcessRequest) (model.ProcessResponse, error) {
	payload := responsesRequest{
		Model:        p.model,
		Instructions: buildInstructions(req),
		Input: []responsesMessage{
			{
				Role: "user",
				Content: []responsesContentPart{
					{Type: "input_text", Text: buildUserPrompt(req)},
				},
			},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return model.ProcessResponse{}, err
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/responses", bytes.NewReader(body))
	if err != nil {
		return model.ProcessResponse{}, err
	}
	httpReq.Header.Set("Authorization", "Bearer "+p.apiKey)
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := p.client.Do(httpReq)
	if err != nil {
		return model.ProcessResponse{}, err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return model.ProcessResponse{}, err
	}
	if resp.StatusCode >= 300 {
		return model.ProcessResponse{}, fmt.Errorf("openai status %d: %s", resp.StatusCode, string(respBody))
	}

	var apiResp responsesAPIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return model.ProcessResponse{}, fmt.Errorf("decode openai response: %w", err)
	}

	text := collectOutputText(apiResp)
	if text == "" {
		return model.ProcessResponse{}, fmt.Errorf("openai response missing text output")
	}

	var result structuredResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return model.ProcessResponse{}, fmt.Errorf("decode structured result: %w; raw=%s", err, text)
	}

	if strings.TrimSpace(result.Title) == "" {
		result.Title = req.Title
	}
	if strings.TrimSpace(result.Content) == "" {
		result.Content = result.Summary
	}

	return model.ProcessResponse{
		Title:   strings.TrimSpace(result.Title),
		Summary: strings.TrimSpace(result.Summary),
		Content: strings.TrimSpace(result.Content),
		Model:   p.model,
	}, nil
}

func buildInstructions(req model.ProcessRequest) string {
	if strings.TrimSpace(req.SystemPrompt) != "" {
		return req.SystemPrompt
	}
	return "You transform RSS articles into structured reader-friendly output. Return strict JSON with title, summary, content. Preserve facts and links."
}

func buildUserPrompt(req model.ProcessRequest) string {
	parts := []string{
		"Return JSON only with keys: title, summary, content.",
		"Original title: " + req.Title,
		"Original link: " + req.Link,
		"Article content:",
		limitText(req.Content, req.MaxInputChars),
	}

	if strings.TrimSpace(req.TaskPrompt) != "" {
		parts = append(parts, "Task:", req.TaskPrompt)
	} else {
		parts = append(parts,
			"Task:",
			"Rewrite the article into useful RSS output with a clear title, a concise summary, and readable content.",
		)
	}

	return strings.Join(parts, "\n")
}

func collectOutputText(resp responsesAPIResponse) string {
	var builder strings.Builder
	for _, output := range resp.Output {
		for _, part := range output.Content {
			if part.Type == "output_text" || part.Type == "text" {
				builder.WriteString(part.Text)
			}
		}
	}
	return strings.TrimSpace(builder.String())
}

func limitText(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
