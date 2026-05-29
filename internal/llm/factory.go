package llm

import (
	"fmt"
	"os"

	"github.com/lite-claw/lite-claw/internal/config"
)

// NewProvider builds a provider from config.
func NewProvider(cfg *config.Config) (Provider, error) {
	name := cfg.Agent.Provider
	pc, ok := cfg.Provider(name)
	if !ok {
		return nil, fmt.Errorf("unknown provider %q", name)
	}

	model := cfg.Agent.Model
	if pc.Model != "" {
		model = pc.Model
	}

	switch name {
	case "ollama":
		return NewOllama(pc.BaseURL, model)
	case "openai":
		key := pc.APIKey
		if key == "" {
			key = os.Getenv("OPENAI_API_KEY")
		}
		return NewOpenAICompat("openai", pc.BaseURL, key, model), nil
	case "anthropic":
		key := pc.APIKey
		if key == "" {
			key = os.Getenv("ANTHROPIC_API_KEY")
		}
		base := pc.BaseURL
		if base == "" || base == "https://api.anthropic.com" {
			base = os.Getenv("LITE_CLAW_ANTHROPIC_BASE_URL")
			if base == "" {
				return nil, fmt.Errorf("anthropic: set providers.anthropic.baseURL to an OpenAI-compatible proxy, or use ollama/openai")
			}
		}
		return NewOpenAICompat("anthropic", base, key, model), nil
	default:
		key := pc.APIKey
		return NewOpenAICompat(name, pc.BaseURL, key, model), nil
	}
}
