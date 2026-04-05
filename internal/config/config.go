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
	Provider  string `toml:"provider"`
	Model     string `toml:"model"`
	APIKey    string `toml:"api_key"`
	APIKeyEnv string `toml:"api_key_env"`
	BaseURL   string `toml:"base_url"`
	Timeout   string `toml:"timeout"`
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
	if c.LLM.APIKey == "" && c.LLM.APIKeyEnv != "" {
		c.LLM.APIKey = os.Getenv(c.LLM.APIKeyEnv)
	}
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
