package llm

import (
	"testing"

	"github.com/batkiz/rss-gateway/internal/model"
)

func TestBuildTextConfig(t *testing.T) {
	cfg := buildTextConfig(model.OutputSchema{
		Name:         "summary",
		TitleField:   "headline",
		SummaryField: "summary",
		ContentField: "content",
		Fields: []model.OutputField{
			{Name: "headline", Type: "string", Required: true},
			{Name: "summary", Type: "string", Required: true},
			{Name: "content", Type: "string", Required: true},
			{Name: "keywords", Type: "array", Required: false},
		},
	})
	if cfg == nil {
		t.Fatal("expected schema config")
	}
	required := cfg.Format.Schema["required"].([]string)
	if len(required) != 3 {
		t.Fatalf("unexpected required fields: %#v", required)
	}
}

func TestParseStructuredResult(t *testing.T) {
	resp, err := parseStructuredResult(`{"headline":"A","summary":"B","content":"C"}`, model.OutputSchema{
		TitleField:   "headline",
		SummaryField: "summary",
		ContentField: "content",
	})
	if err != nil {
		t.Fatalf("parseStructuredResult error: %v", err)
	}
	if resp.Title != "A" || resp.Summary != "B" || resp.Content != "C" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}
