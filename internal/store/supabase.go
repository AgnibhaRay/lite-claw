package store

import (
	"context"
	"strings"

	"github.com/lite-claw/lite-claw/internal/llm"
	"github.com/lite-claw/lite-claw/internal/supabase"
)

// SupabaseStore persists sessions via Supabase PostgREST.
type SupabaseStore struct {
	client *supabase.Client
}

func NewSupabaseStore(client *supabase.Client) *SupabaseStore {
	return &SupabaseStore{client: client}
}

func (s *SupabaseStore) Load(ctx context.Context, sessionID string) ([]llm.Message, error) {
	rows, err := s.client.Messages().List(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	msgs := make([]llm.Message, 0, len(rows))
	for _, row := range rows {
		msgs = append(msgs, llm.Message{
			Role:       llm.Role(row.Role),
			Content:    row.Content,
			ToolCallID: row.ToolCallID,
			ToolName:   row.ToolName,
		})
	}
	return TrimHistory(msgs, MaxHistory), nil
}

func (s *SupabaseStore) Save(ctx context.Context, sessionID string, msgs []llm.Message) error {
	msgs = TrimHistory(msgs, MaxHistory)

	channel, sender := parseSessionID(sessionID)
	if err := s.client.Sessions().Upsert(ctx, supabase.SessionRow{
		ID:      sessionID,
		Channel: channel,
		Sender:  sender,
	}); err != nil {
		return err
	}

	rows := make([]supabase.MessageRow, len(msgs))
	for i, m := range msgs {
		rows[i] = supabase.MessageRow{
			SessionID:  sessionID,
			Role:       string(m.Role),
			Content:    m.Content,
			ToolCallID: m.ToolCallID,
			ToolName:   m.ToolName,
			Position:   i,
		}
	}
	return s.client.Messages().Replace(ctx, sessionID, rows)
}

func parseSessionID(sessionID string) (channel, sender string) {
	if i := strings.Index(sessionID, ":"); i > 0 {
		return sessionID[:i], sessionID[i+1:]
	}
	if sessionID == "cli" {
		return "cli", ""
	}
	return "unknown", sessionID
}

// SupabaseMemoryStore persists agent memory in Supabase.
type SupabaseMemoryStore struct {
	client *supabase.Client
}

func NewSupabaseMemoryStore(client *supabase.Client) *SupabaseMemoryStore {
	return &SupabaseMemoryStore{client: client}
}

func (s *SupabaseMemoryStore) Set(ctx context.Context, sessionID, key, value string) error {
	return s.client.Memories().Set(ctx, sessionID, key, value)
}

func (s *SupabaseMemoryStore) Get(ctx context.Context, sessionID, key string) (string, error) {
	return s.client.Memories().Get(ctx, sessionID, key)
}

func (s *SupabaseMemoryStore) ListKeys(ctx context.Context, sessionID string) ([]string, error) {
	return s.client.Memories().ListKeys(ctx, sessionID)
}

// SupabaseMessageLogger writes channel traffic to message_log.
type SupabaseMessageLogger struct {
	client *supabase.Client
}

func NewSupabaseMessageLogger(client *supabase.Client) *SupabaseMessageLogger {
	return &SupabaseMessageLogger{client: client}
}

func (l *SupabaseMessageLogger) Log(ctx context.Context, entry MessageLogEntry) error {
	return l.client.MessageLog().Insert(ctx, supabase.MessageLogRow{
		SessionID: entry.SessionID,
		Channel:   entry.Channel,
		Direction: entry.Direction,
		Sender:    entry.Sender,
		Content:   entry.Content,
	})
}
