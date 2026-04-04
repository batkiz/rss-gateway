package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAppliesModeSchemaDefaults(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	data := []byte(`
[llm]
provider = "openai"
api_key = "test"

[modes.summary]
system_prompt = "x"

[[sources]]
id = "demo"
url = "https://example.com/rss"

[sources.pipeline]
mode = "summary"
`)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load error: %v", err)
	}

	mode := cfg.Modes["summary"]
	if mode.OutputSchema.TitleField != "title" || mode.OutputSchema.SummaryField != "summary" || mode.OutputSchema.ContentField != "content" {
		t.Fatalf("unexpected schema defaults: %+v", mode.OutputSchema)
	}
}
