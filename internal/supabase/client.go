package supabase

import (
	"fmt"
	"os"
	"strings"

	supabase "github.com/supabase-community/supabase-go"
)

// Client wraps the Supabase REST SDK with lite-claw repositories.
type Client struct {
	raw     *supabase.Client
	baseURL string
}

// NewClient builds a Supabase client from config, with env fallbacks.
func NewClient(cfg Config) (*Client, error) {
	url := firstNonEmpty(cfg.URL, os.Getenv("SUPABASE_URL"))
	key := firstNonEmpty(cfg.ServiceKey, os.Getenv("SUPABASE_SERVICE_ROLE_KEY"))
	if key == "" {
		key = firstNonEmpty(cfg.AnonKey, os.Getenv("SUPABASE_ANON_KEY"))
	}
	if url == "" || key == "" {
		return nil, fmt.Errorf("supabase: set database.supabase.url and serviceKey (or SUPABASE_URL + SUPABASE_SERVICE_ROLE_KEY)")
	}

	raw, err := supabase.NewClient(url, key, &supabase.ClientOptions{})
	if err != nil {
		return nil, fmt.Errorf("supabase client: %w", err)
	}
	return &Client{raw: raw, baseURL: strings.TrimRight(url, "/")}, nil
}

// Ping verifies connectivity by querying the sessions table.
func (c *Client) Ping() error {
	_, err := c.raw.From("sessions").Select("id", "", false).Limit(1, "").ExecuteTo(&[]map[string]any{})
	if err != nil {
		return fmt.Errorf("supabase ping failed (did you run migrations?): %w", err)
	}
	return nil
}

// Raw exposes the underlying SDK for advanced use.
func (c *Client) Raw() *supabase.Client {
	return c.raw
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}
