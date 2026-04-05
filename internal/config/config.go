package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Server  ServerConfig          `toml:"server"`
	Storage StorageConfig         `toml:"storage"`
	LLM     LLMConfig             `toml:"llm"`
	Modes   map[string]ModeConfig `toml:"modes"`
	Sources []Source              `toml:"sources"`
}

type ServerConfig struct {
	Addr string `toml:"addr"`
}

type StorageConfig struct {
	Path string `toml:"path"`
}

type LLMConfig struct {
	Provider string `toml:"provider"`
	Model    string `toml:"model"`
	APIKey   string `toml:"api_key"`
	BaseURL  string `toml:"base_url"`
	Timeout  string `toml:"timeout"`
}

type Source struct {
	ID              string         `toml:"id"`
	Name            string         `toml:"name"`
	URL             string         `toml:"url"`
	RefreshInterval Duration       `toml:"refresh_interval"`
	Enabled         *bool          `toml:"enabled"`
	MaxItems        int            `toml:"max_items"`
	Pipeline        PipelineConfig `toml:"pipeline"`
}

type PipelineConfig struct {
	Mode               string   `toml:"mode"`
	SystemPrompt       string   `toml:"system_prompt"`
	TaskPrompt         string   `toml:"task_prompt"`
	MaxInputChars      int      `toml:"max_input_chars"`
	ExtractFullContent bool     `toml:"extract_full_content"`
	Temperature        *float64 `toml:"temperature"`
	MaxOutputTokens    int      `toml:"max_output_tokens"`
}

type ModeConfig struct {
	SystemPrompt    string             `toml:"system_prompt"`
	TaskPrompt      string             `toml:"task_prompt"`
	Temperature     *float64           `toml:"temperature"`
	MaxOutputTokens int                `toml:"max_output_tokens"`
	OutputSchema    OutputSchemaConfig `toml:"output_schema"`
}

type OutputSchemaConfig struct {
	Name         string              `toml:"name"`
	TitleField   string              `toml:"title_field"`
	SummaryField string              `toml:"summary_field"`
	ContentField string              `toml:"content_field"`
	ExtraFields  []OutputFieldConfig `toml:"extra_fields"`
}

type OutputFieldConfig struct {
	Name        string `toml:"name"`
	Type        string `toml:"type"`
	Description string `toml:"description"`
	Required    *bool  `toml:"required"`
}

type Duration struct {
	time.Duration
}

const defaultConfigTOML = `[server]
addr = ":8080"

[storage]
path = "data/rss-gateway.db"

[llm]
provider = "openai"
model = "gpt-4.1-mini"
api_key = ""
base_url = "https://api.openai.com/v1"
timeout = "60s"

[modes.summary]
system_prompt = "You transform RSS articles into concise reader-friendly summaries. Return strict JSON with title, summary, content. Preserve facts and links. Keep output compact and useful."
temperature = 0.2
max_output_tokens = 900
task_prompt = """
1. Keep or lightly rewrite the title for clarity.
2. Write a short summary in 3 to 5 sentences.
3. Produce concise output content suitable for an RSS reader.
"""

[modes.summary.output_schema]
name = "summary"
title_field = "title"
summary_field = "summary"
content_field = "content"

[modes.translate_zh]
system_prompt = "You transform RSS articles into Chinese reader-friendly output. Return strict JSON with title, summary, content. Preserve facts and links. Write concise simplified Chinese."
temperature = 0.3
max_output_tokens = 1200
task_prompt = """
1. Rewrite the title in simplified Chinese.
2. Write a short Chinese summary in 3 to 5 sentences.
3. Produce Chinese output content suitable for an RSS reader.
"""

[modes.translate_zh.output_schema]
name = "translate_zh"
title_field = "title"
summary_field = "summary"
content_field = "content"

[[modes.translate_zh.output_schema.extra_fields]]
name = "keywords"
type = "array"
description = "A short list of important keywords."
required = false

[[sources]]
id = "hackernews-summary"
name = "Hacker News Summary"
url = "https://news.ycombinator.com/rss"
refresh_interval = "10m"
enabled = true
max_items = 15

[sources.pipeline]
mode = "summary"
max_input_chars = 6000
extract_full_content = true

[[sources]]
id = "lobsters-translate"
name = "Lobsters Chinese"
url = "https://lobste.rs/rss"
refresh_interval = "15m"
enabled = true
max_items = 15

[sources.pipeline]
mode = "translate_zh"
max_input_chars = 6000
max_output_tokens = 1400
`

func (d *Duration) UnmarshalText(text []byte) error {
	raw := strings.TrimSpace(string(text))
	if raw == "" {
		d.Duration = 0
		return nil
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}

func EnsureFile(path string) (bool, error) {
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".toml" {
		return false, fmt.Errorf("unsupported config format %q: only .toml is supported", ext)
	}

	if _, err := os.Stat(path); err == nil {
		return false, nil
	} else if !os.IsNotExist(err) {
		return false, err
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return false, err
	}
	if err := os.WriteFile(path, []byte(defaultConfigTOML), 0o644); err != nil {
		return false, err
	}
	return true, nil
}

func Load(path string) (Config, error) {
	if ext := strings.ToLower(filepath.Ext(path)); ext != ".toml" {
		return Config{}, fmt.Errorf("unsupported config format %q: only .toml is supported", ext)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, err
	}

	cfg.applyDefaults()
	if err := cfg.validateAndResolve(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c *Config) applyDefaults() {
	if c.Server.Addr == "" {
		c.Server.Addr = ":8080"
	}
	if c.Storage.Path == "" {
		c.Storage.Path = "data/rss-gateway.db"
	}
	if c.LLM.Provider == "" {
		c.LLM.Provider = "openai"
	}
	if c.LLM.BaseURL == "" {
		c.LLM.BaseURL = "https://api.openai.com/v1"
	}
	if c.LLM.Timeout == "" {
		c.LLM.Timeout = "60s"
	}
	for i := range c.Sources {
		if c.Sources[i].RefreshInterval.Duration == 0 {
			c.Sources[i].RefreshInterval.Duration = 30 * time.Minute
		}
		if c.Sources[i].MaxItems <= 0 {
			c.Sources[i].MaxItems = 20
		}
		if c.Sources[i].Pipeline.MaxInputChars <= 0 {
			c.Sources[i].Pipeline.MaxInputChars = 8000
		}
	}
	for key, mode := range c.Modes {
		if mode.OutputSchema.Name == "" {
			mode.OutputSchema.Name = key
		}
		if mode.OutputSchema.TitleField == "" {
			mode.OutputSchema.TitleField = "title"
		}
		if mode.OutputSchema.SummaryField == "" {
			mode.OutputSchema.SummaryField = "summary"
		}
		if mode.OutputSchema.ContentField == "" {
			mode.OutputSchema.ContentField = "content"
		}
		c.Modes[key] = mode
	}
}

func (c *Config) validateAndResolve() error {
	for _, source := range c.Sources {
		if source.ID == "" {
			return fmt.Errorf("source id is required")
		}
		if source.URL == "" {
			return fmt.Errorf("source %s url is required", source.ID)
		}
		if source.Pipeline.Mode == "" {
			return fmt.Errorf("source %s pipeline.mode is required", source.ID)
		}
		if _, ok := c.Modes[source.Pipeline.Mode]; !ok && source.Pipeline.SystemPrompt == "" && source.Pipeline.TaskPrompt == "" {
			return fmt.Errorf("source %s references undefined mode %q", source.ID, source.Pipeline.Mode)
		}
	}
	for name, mode := range c.Modes {
		if err := validateOutputSchema(name, mode.OutputSchema); err != nil {
			return err
		}
	}
	return nil
}

func validateOutputSchema(modeName string, schema OutputSchemaConfig) error {
	if schema.TitleField == "" || schema.SummaryField == "" || schema.ContentField == "" {
		return fmt.Errorf("mode %s output_schema title_field, summary_field, and content_field are required", modeName)
	}
	seen := map[string]struct{}{
		schema.TitleField:   {},
		schema.SummaryField: {},
		schema.ContentField: {},
	}
	for _, field := range schema.ExtraFields {
		if field.Name == "" {
			return fmt.Errorf("mode %s has output_schema extra field with empty name", modeName)
		}
		if _, ok := seen[field.Name]; ok {
			return fmt.Errorf("mode %s has duplicate output schema field %q", modeName, field.Name)
		}
		seen[field.Name] = struct{}{}
	}
	return nil
}
