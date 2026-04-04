package config

import (
	"fmt"
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server  ServerConfig          `yaml:"server"`
	Storage StorageConfig         `yaml:"storage"`
	LLM     LLMConfig             `yaml:"llm"`
	Modes   map[string]ModeConfig `yaml:"modes"`
	Sources []Source              `yaml:"sources"`
}

type ServerConfig struct {
	Addr string `yaml:"addr"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type LLMConfig struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	APIKey    string `yaml:"api_key"`
	APIKeyEnv string `yaml:"api_key_env"`
	BaseURL   string `yaml:"base_url"`
	Timeout   string `yaml:"timeout"`
}

type Source struct {
	ID              string         `yaml:"id"`
	Name            string         `yaml:"name"`
	URL             string         `yaml:"url"`
	RefreshInterval Duration       `yaml:"refresh_interval"`
	Enabled         *bool          `yaml:"enabled"`
	MaxItems        int            `yaml:"max_items"`
	Pipeline        PipelineConfig `yaml:"pipeline"`
}

type PipelineConfig struct {
	Mode          string `yaml:"mode"`
	SystemPrompt  string `yaml:"system_prompt"`
	TaskPrompt    string `yaml:"task_prompt"`
	MaxInputChars int    `yaml:"max_input_chars"`
}

type ModeConfig struct {
	SystemPrompt string `yaml:"system_prompt"`
	TaskPrompt   string `yaml:"task_prompt"`
}

type Duration struct {
	time.Duration
}

func (d *Duration) UnmarshalYAML(value *yaml.Node) error {
	var raw string
	if err := value.Decode(&raw); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil {
		return fmt.Errorf("parse duration %q: %w", raw, err)
	}
	d.Duration = parsed
	return nil
}

func Load(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, err
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
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
}

func (c *Config) validateAndResolve() error {
	if c.LLM.APIKey == "" && c.LLM.APIKeyEnv != "" {
		c.LLM.APIKey = os.Getenv(c.LLM.APIKeyEnv)
	}
	if c.LLM.Provider == "openai" && c.LLM.APIKey == "" {
		return fmt.Errorf("llm api key is required")
	}
	if len(c.Sources) == 0 {
		return fmt.Errorf("at least one source is required")
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
	return nil
}
