package store

import (
	"fmt"

	"github.com/lite-claw/lite-claw/internal/config"
	"github.com/lite-claw/lite-claw/internal/supabase"
)

// New creates the configured session store.
func New(cfg *config.Config) (Store, error) {
	switch cfg.Database.Driver {
	case "", "file":
		return NewFileStore(cfg.Gateway.DataDir)
	case "supabase":
		client, err := supabase.NewClient(toSupabaseConfig(cfg.Database.Supabase))
		if err != nil {
			return nil, err
		}
		return NewSupabaseStore(client), nil
	default:
		return nil, fmt.Errorf("unknown database driver %q", cfg.Database.Driver)
	}
}

// NewMemoryStore returns a memory backend when Supabase is enabled, else nil.
func NewMemoryStore(cfg *config.Config) (MemoryStore, error) {
	if cfg.Database.Driver != "supabase" {
		return nil, nil
	}
	client, err := supabase.NewClient(toSupabaseConfig(cfg.Database.Supabase))
	if err != nil {
		return nil, err
	}
	return NewSupabaseMemoryStore(client), nil
}

// NewMessageLogger returns a message logger when Supabase is enabled, else nil.
func NewMessageLogger(cfg *config.Config) (MessageLogger, error) {
	if cfg.Database.Driver != "supabase" {
		return nil, nil
	}
	client, err := supabase.NewClient(toSupabaseConfig(cfg.Database.Supabase))
	if err != nil {
		return nil, err
	}
	return NewSupabaseMessageLogger(client), nil
}

func toSupabaseConfig(c config.SupabaseConfig) supabase.Config {
	return supabase.Config{
		URL:        c.URL,
		AnonKey:    c.AnonKey,
		ServiceKey: c.ServiceKey,
	}
}
