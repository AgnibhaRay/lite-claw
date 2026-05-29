package store

import (
	"context"

	"github.com/lite-claw/lite-claw/internal/llm"
)

// Store persists conversation history per session.
type Store interface {
	Load(ctx context.Context, sessionID string) ([]llm.Message, error)
	Save(ctx context.Context, sessionID string, msgs []llm.Message) error
}

// MemoryStore persists long-term key/value facts.
type MemoryStore interface {
	Set(ctx context.Context, sessionID, key, value string) error
	Get(ctx context.Context, sessionID, key string) (string, error)
	ListKeys(ctx context.Context, sessionID string) ([]string, error)
}

// MessageLogger records inbound/outbound channel traffic.
type MessageLogger interface {
	Log(ctx context.Context, entry MessageLogEntry) error
}

// MessageLogEntry is one channel message audit row.
type MessageLogEntry struct {
	SessionID string
	Channel   string
	Direction string // in | out
	Sender    string
	Content   string
}

// SessionMeta describes a conversation session.
type SessionMeta struct {
	ID      string
	Channel string
	Sender  string
	Title   string
}

const MaxHistory = 40

func TrimHistory(msgs []llm.Message, max int) []llm.Message {
	if max <= 0 {
		max = MaxHistory
	}
	if len(msgs) <= max {
		return msgs
	}
	return msgs[len(msgs)-max:]
}

func FilterPersistable(msgs []llm.Message) []llm.Message {
	out := make([]llm.Message, 0, len(msgs))
	for _, m := range msgs {
		if m.Role == llm.RoleSystem {
			continue
		}
		out = append(out, m)
	}
	return TrimHistory(out, MaxHistory)
}
