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
	Model           string             `json:"model"`
	Instructions    string             `json:"instructions,omitempty"`
	Input           []responsesMessage `json:"input"`
	Temperature     *float64           `json:"temperature,omitempty"`
	MaxOutputTokens int                `json:"max_output_tokens,omitempty"`
	Text            *responsesText     `json:"text,omitempty"`
}

type responsesText struct {
	Format responsesFormat `json:"format"`
}

type responsesFormat struct {
	Type   string         `json:"type"`
	Name   string         `json:"name"`
	Strict bool           `json:"strict"`
	Schema map[string]any `json:"schema"`
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
		Model:           p.model,
		Instructions:    buildInstructions(req),
		Input:           []responsesMessage{{Role: "user", Content: []responsesContentPart{{Type: "input_text", Text: buildUserPrompt(req)}}}},
		Temperature:     req.Temperature,
		MaxOutputTokens: req.MaxOutputTokens,
		Text:            buildTextConfig(req.OutputSchema),
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

	parsed, err := parseStructuredResult(text, req.OutputSchema)
	if err != nil {
		return model.ProcessResponse{}, err
	}
	parsed.Model = p.model
	parsed.OutputJSON = text
	return parsed, nil
}

func buildInstructions(req model.ProcessRequest) string {
	if strings.TrimSpace(req.SystemPrompt) != "" {
		return req.SystemPrompt
	}
	return "You transform RSS articles into structured reader-friendly output. Preserve facts and links."
}

func buildUserPrompt(req model.ProcessRequest) string {
	parts := []string{
		"Return output that matches the configured JSON schema.",
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

func buildTextConfig(schema model.OutputSchema) *responsesText {
	if len(schema.Fields) == 0 {
		return nil
	}

	properties := make(map[string]any, len(schema.Fields))
	required := make([]string, 0, len(schema.Fields))
	for _, field := range schema.Fields {
		fieldType := field.Type
		if fieldType == "" {
			fieldType = "string"
		}
		property := map[string]any{
			"type":        fieldType,
			"description": field.Description,
		}
		if fieldType == "array" {
			property["items"] = map[string]any{"type": "string"}
		}
		properties[field.Name] = property
		if field.Required {
			required = append(required, field.Name)
		}
	}

	return &responsesText{
		Format: responsesFormat{
			Type:   "json_schema",
			Name:   schema.Name,
			Strict: true,
			Schema: map[string]any{
				"type":                 "object",
				"properties":           properties,
				"required":             required,
				"additionalProperties": false,
			},
		},
	}
}

func parseStructuredResult(raw string, schema model.OutputSchema) (model.ProcessResponse, error) {
	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		return model.ProcessResponse{}, fmt.Errorf("decode structured result: %w; raw=%s", err, raw)
	}

	title := stringValue(payload[schema.TitleField])
	summary := stringValue(payload[schema.SummaryField])
	content := stringValue(payload[schema.ContentField])
	if strings.TrimSpace(content) == "" {
		content = summary
	}

	return model.ProcessResponse{
		Title:   strings.TrimSpace(title),
		Summary: strings.TrimSpace(summary),
		Content: strings.TrimSpace(content),
	}, nil
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

func stringValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case []any:
		parts := make([]string, 0, len(v))
		for _, item := range v {
			parts = append(parts, stringValue(item))
		}
		return strings.Join(parts, ", ")
	default:
		if value == nil {
			return ""
		}
		return fmt.Sprint(value)
	}
}

func limitText(text string, maxChars int) string {
	text = strings.TrimSpace(text)
	if maxChars <= 0 || len(text) <= maxChars {
		return text
	}
	return text[:maxChars]
}
