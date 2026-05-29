package supabase

import (
	"context"
	"fmt"
	"strings"

	postgrest "github.com/supabase-community/postgrest-go"
)

// SessionsRepo manages conversation sessions.
type SessionsRepo struct {
	client *Client
}

func (c *Client) Sessions() *SessionsRepo { return &SessionsRepo{client: c} }

func (r *SessionsRepo) Upsert(ctx context.Context, row SessionRow) error {
	if row.ID == "" {
		return fmt.Errorf("session id required")
	}
	if row.Channel == "" {
		row.Channel = "unknown"
	}
	_, _, err := r.client.raw.From("sessions").Upsert(row, "id", "", "").Execute()
	if err != nil {
		return fmt.Errorf("sessions upsert: %w", err)
	}
	return nil
}

func (r *SessionsRepo) Get(ctx context.Context, id string) (*SessionRow, error) {
	var rows []SessionRow
	_, err := r.client.raw.From("sessions").Select("*", "", false).Eq("id", id).ExecuteTo(&rows)
	if err != nil {
		return nil, fmt.Errorf("sessions get: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// MessagesRepo manages session message history.
type MessagesRepo struct {
	client *Client
}

func (c *Client) Messages() *MessagesRepo { return &MessagesRepo{client: c} }

func (r *MessagesRepo) List(ctx context.Context, sessionID string) ([]MessageRow, error) {
	var rows []MessageRow
	_, err := r.client.raw.From("session_messages").
		Select("*", "", false).
		Eq("session_id", sessionID).
		Order("position", &postgrest.OrderOpts{Ascending: true}).
		ExecuteTo(&rows)
	if err != nil {
		return nil, fmt.Errorf("messages list: %w", err)
	}
	return rows, nil
}

func (r *MessagesRepo) Replace(ctx context.Context, sessionID string, messages []MessageRow) error {
	_, _, err := r.client.raw.From("session_messages").Delete("", "").Eq("session_id", sessionID).Execute()
	if err != nil {
		return fmt.Errorf("messages delete: %w", err)
	}
	if len(messages) == 0 {
		return nil
	}
	for i := range messages {
		messages[i].SessionID = sessionID
		messages[i].Position = i
	}
	_, _, err = r.client.raw.From("session_messages").Insert(messages, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("messages insert: %w", err)
	}
	return nil
}

// MemoriesRepo manages long-term agent memory.
type MemoriesRepo struct {
	client *Client
}

func (c *Client) Memories() *MemoriesRepo { return &MemoriesRepo{client: c} }

func (r *MemoriesRepo) Set(ctx context.Context, sessionID, key, value string) error {
	row := MemoryRow{SessionID: sessionID, Key: key, Value: value}
	_, _, err := r.client.raw.From("memories").Upsert(row, "session_id,key", "", "").Execute()
	if err != nil {
		return fmt.Errorf("memories upsert: %w", err)
	}
	return nil
}

func (r *MemoriesRepo) Get(ctx context.Context, sessionID, key string) (string, error) {
	q := r.client.raw.From("memories").Select("value", "", false).Eq("key", key)
	if sessionID == "" {
		q = q.Is("session_id", "null")
	} else {
		q = q.Eq("session_id", sessionID)
	}
	var rows []MemoryRow
	_, err := q.ExecuteTo(&rows)
	if err != nil {
		return "", fmt.Errorf("memories get: %w", err)
	}
	if len(rows) == 0 {
		return "", fmt.Errorf("no memory for key %q", key)
	}
	return rows[0].Value, nil
}

func (r *MemoriesRepo) ListKeys(ctx context.Context, sessionID string) ([]string, error) {
	q := r.client.raw.From("memories").Select("key", "", false)
	if sessionID == "" {
		q = q.Is("session_id", "null")
	} else {
		q = q.Eq("session_id", sessionID)
	}
	var rows []MemoryRow
	_, err := q.ExecuteTo(&rows)
	if err != nil {
		return nil, fmt.Errorf("memories list: %w", err)
	}
	keys := make([]string, 0, len(rows))
	for _, row := range rows {
		keys = append(keys, row.Key)
	}
	return keys, nil
}

// ContactsRepo manages channel contacts.
type ContactsRepo struct {
	client *Client
}

func (c *Client) Contacts() *ContactsRepo { return &ContactsRepo{client: c} }

func (r *ContactsRepo) Upsert(ctx context.Context, row ContactRow) error {
	if row.Channel == "" || row.ExternalID == "" {
		return fmt.Errorf("channel and external_id required")
	}
	_, _, err := r.client.raw.From("contacts").Upsert(row, "channel,external_id", "", "").Execute()
	if err != nil {
		return fmt.Errorf("contacts upsert: %w", err)
	}
	return nil
}

func (r *ContactsRepo) Find(ctx context.Context, channel, externalID string) (*ContactRow, error) {
	var rows []ContactRow
	_, err := r.client.raw.From("contacts").
		Select("*", "", false).
		Eq("channel", channel).
		Eq("external_id", externalID).
		ExecuteTo(&rows)
	if err != nil {
		return nil, fmt.Errorf("contacts find: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// MessageLogRepo records inbound/outbound traffic.
type MessageLogRepo struct {
	client *Client
}

func (c *Client) MessageLog() *MessageLogRepo { return &MessageLogRepo{client: c} }

func (r *MessageLogRepo) Insert(ctx context.Context, row MessageLogRow) error {
	row.Direction = strings.ToLower(row.Direction)
	if row.Direction != "in" && row.Direction != "out" {
		return fmt.Errorf("direction must be in or out")
	}
	_, _, err := r.client.raw.From("message_log").Insert(row, false, "", "", "").Execute()
	if err != nil {
		return fmt.Errorf("message_log insert: %w", err)
	}
	return nil
}
