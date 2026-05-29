package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the root lite-claw configuration.
type Config struct {
	Agent     AgentConfig               `json:"agent"`
	Gateway   GatewayConfig             `json:"gateway"`
	Database  DatabaseConfig            `json:"database"`
	Channels  ChannelsConfig            `json:"channels"`
	Providers map[string]ProviderConfig `json:"providers"`
}

type AgentConfig struct {
	Provider string `json:"provider"` // ollama, openai, anthropic
	Model    string `json:"model"`
	System   string `json:"system"`
	MaxTurns int    `json:"maxTurns"`
	Workspace string `json:"workspace"`
}

type GatewayConfig struct {
	DataDir string `json:"dataDir"`
}

type DatabaseConfig struct {
	Driver   string         `json:"driver"` // file | supabase
	Supabase SupabaseConfig `json:"supabase"`
}

type SupabaseConfig struct {
	URL        string `json:"url"`
	AnonKey    string `json:"anonKey"`
	ServiceKey string `json:"serviceKey"`
}

type ChannelsConfig struct {
	WhatsApp WhatsAppConfig `json:"whatsapp"`
}

type WhatsAppConfig struct {
	Enabled   bool     `json:"enabled"`
	AllowFrom []string `json:"allowFrom"` // E.164 like +15551234567, or * for all
	SelfChat  bool     `json:"selfChat"`  // respond to messages from linked device
}

type ProviderConfig struct {
	BaseURL string `json:"baseURL"`
	APIKey  string `json:"apiKey"`
	Model   string `json:"model"` // default model override
}

// Default returns sensible defaults for local Ollama + WhatsApp.
func Default() *Config {
	home, _ := os.UserHomeDir()
	dataDir := filepath.Join(home, ".lite-claw")
	return &Config{
		Agent: AgentConfig{
			Provider:  "ollama",
			Model:     "llama3.2",
			MaxTurns:  12,
			Workspace: ".",
			System: `You are lite-claw, a helpful personal assistant running locally.
You have tools to read/write files, list directories, run shell commands, and remember facts.
Be concise and practical. When using shell, prefer safe, non-destructive commands unless the user clearly asks otherwise.`,
		},
		Gateway: GatewayConfig{DataDir: dataDir},
		Database: DatabaseConfig{
			Driver: "file",
		},
		Channels: ChannelsConfig{
			WhatsApp: WhatsAppConfig{
				Enabled:   true,
				AllowFrom: []string{"*"},
			},
		},
		Providers: map[string]ProviderConfig{
			"ollama": {
				BaseURL: "http://127.0.0.1:11434",
			},
			"openai": {
				BaseURL: "https://api.openai.com/v1",
			},
			"anthropic": {
				BaseURL: "https://api.anthropic.com",
			},
		},
	}
}

// Load reads config from path or returns defaults if missing.
func Load(path string) (*Config, error) {
	if path == "" {
		path = DefaultPath()
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			cfg := Default()
			if err := cfg.Save(path); err != nil {
				return cfg, fmt.Errorf("create default config: %w", err)
			}
			return cfg, nil
		}
		return nil, err
	}
	cfg := Default()
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.Gateway.DataDir == "" {
		home, _ := os.UserHomeDir()
		cfg.Gateway.DataDir = filepath.Join(home, ".lite-claw")
	}
	if cfg.Agent.MaxTurns <= 0 {
		cfg.Agent.MaxTurns = 12
	}
	if cfg.Providers == nil {
		cfg.Providers = Default().Providers
	}
	return cfg, nil
}

func (c *Config) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func DefaultPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "lite-claw.json"
	}
	return filepath.Join(home, ".lite-claw", "config.json")
}

func (c *Config) Provider(name string) (ProviderConfig, bool) {
	if name == "" {
		name = c.Agent.Provider
	}
	p, ok := c.Providers[name]
	return p, ok
}
