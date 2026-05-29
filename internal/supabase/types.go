package supabase

// Config holds Supabase connection settings.
type Config struct {
	URL        string `json:"url"`
	AnonKey    string `json:"anonKey"`
	ServiceKey string `json:"serviceKey"`
}

// SessionRow maps to public.sessions.
type SessionRow struct {
	ID        string         `json:"id"`
	Channel   string         `json:"channel"`
	Sender    string         `json:"sender,omitempty"`
	Title     string         `json:"title,omitempty"`
	Metadata  map[string]any `json:"metadata,omitempty"`
	UpdatedAt string         `json:"updated_at,omitempty"`
}

// MessageRow maps to public.session_messages.
type MessageRow struct {
	ID         int64  `json:"id,omitempty"`
	SessionID  string `json:"session_id"`
	Role       string `json:"role"`
	Content    string `json:"content"`
	ToolCallID string `json:"tool_call_id,omitempty"`
	ToolName   string `json:"tool_name,omitempty"`
	Position   int    `json:"position"`
}

// MemoryRow maps to public.memories.
type MemoryRow struct {
	ID        int64  `json:"id,omitempty"`
	SessionID string `json:"session_id,omitempty"`
	Key       string `json:"key"`
	Value     string `json:"value"`
}

// ContactRow maps to public.contacts.
type ContactRow struct {
	ID          int64          `json:"id,omitempty"`
	Channel     string         `json:"channel"`
	ExternalID  string         `json:"external_id"`
	DisplayName string         `json:"display_name,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}

// MessageLogRow maps to public.message_log.
type MessageLogRow struct {
	SessionID string `json:"session_id,omitempty"`
	Channel   string `json:"channel"`
	Direction string `json:"direction"`
	Sender    string `json:"sender,omitempty"`
	Content   string `json:"content"`
}
