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

func TestEnsureFileWritesDefaultConfig(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "configs", "config.toml")

	created, err := EnsureFile(path)
	if err != nil {
		t.Fatalf("EnsureFile error: %v", err)
	}
	if !created {
		t.Fatalf("expected config file to be created")
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("Load created config: %v", err)
	}
	if cfg.Server.Addr != ":8080" {
		t.Fatalf("unexpected server addr: %s", cfg.Server.Addr)
	}
	if len(cfg.Sources) == 0 {
		t.Fatalf("expected default sources to be present")
	}

	created, err = EnsureFile(path)
	if err != nil {
		t.Fatalf("EnsureFile on existing config error: %v", err)
	}
	if created {
		t.Fatalf("expected existing config to be left untouched")
	}
}
